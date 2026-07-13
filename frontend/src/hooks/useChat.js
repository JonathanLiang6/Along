import { useState, useEffect, useRef, useCallback } from 'react'
import {
  CreateConversation,
  ListConversations,
  RenameConversation,
  DeleteConversation,
  GetConversationMessages,
  SendMessageStreamInConversation,
} from '../../wailsjs/go/main/App'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

// 流式事件名称
const STREAM_EVENT = 'chat-stream'

// 流式消息的临时 ID
export const STREAMING_MSG_ID = '__streaming__'

// 修复未闭合的代码块，避免 Markdown 闪烁
function fixStreamingMarkdown(text) {
  if (!text) return text
  const codeFenceCount = (text.match(/```/g) || []).length
  if (codeFenceCount % 2 === 1) {
    return text + '\n```'
  }
  return text
}

export { fixStreamingMarkdown }

/**
 * 聊天页面的核心 hook：封装对话列表、消息、流式接收等全部状态与逻辑。
 *
 * 设计要点：
 *  - 事件监听器只注册一次，通过 ref 读取最新状态，避免反复订阅与闭包过期。
 *  - 用 streamingConvIdRef 锁定当前流式所属对话，切换对话不会丢失/串流。
 *  - 初始化只跑一次，不会因 isStreaming 切换而重跑。
 */
export function useChat() {
  const [conversations, setConversations] = useState([])
  const [activeConvId, setActiveConvId] = useState(null)
  const [messages, setMessages] = useState([])
  const [inputValue, setInputValue] = useState('')
  const [isStreaming, setIsStreaming] = useState(false)
  const [streamingContent, setStreamingContent] = useState('')
  const [loadingConv, setLoadingConv] = useState(false)
  const [error, setError] = useState(null)

  // 用 ref 镜像 activeConvId，事件回调里读取最新值
  const activeConvIdRef = useRef(null)
  // 当前流式所属对话 ID，避免切换对话后旧流写入新对话
  const streamingConvIdRef = useRef(null)
  // 流式缓冲（与 streamingContent 同步，回调读取避免闭包过期）
  const streamBufferRef = useRef('')

  useEffect(() => {
    activeConvIdRef.current = activeConvId
  }, [activeConvId])

  const loadConversations = useCallback(async () => {
    try {
      const list = await ListConversations()
      const ok = Array.isArray(list) ? list : []
      setConversations(ok)
      return ok
    } catch (err) {
      console.error('加载对话列表失败:', err)
      setError('加载对话列表失败')
      return []
    }
  }, [])

  const loadMessages = useCallback(async (id) => {
    if (!id) {
      setMessages([])
      return
    }
    setLoadingConv(true)
    try {
      const msgs = await GetConversationMessages(id)
      setMessages(Array.isArray(msgs) ? msgs : [])
    } catch (err) {
      console.error('加载对话消息失败:', err)
      setError('加载消息失败')
      setMessages([])
    } finally {
      setLoadingConv(false)
    }
  }, [])

  const switchConversation = useCallback(
    async (id) => {
      if (!id) return
      // 流式中允许切换查看历史，但本对话的流式不再回写
      setActiveConvId(id)
      activeConvIdRef.current = id
      setStreamingContent('')
      streamBufferRef.current = ''
      await loadMessages(id)
    },
    [loadMessages]
  )

  const newConversation = useCallback(async () => {
    try {
      const conv = await CreateConversation('新对话')
      await loadConversations()
      setActiveConvId(conv.id)
      activeConvIdRef.current = conv.id
      setMessages([])
      setStreamingContent('')
      streamBufferRef.current = ''
      return conv
    } catch (err) {
      console.error('创建对话失败:', err)
      setError('创建对话失败')
      return null
    }
  }, [loadConversations])

  const deleteConversation = useCallback(
    async (id) => {
      try {
        await DeleteConversation(id)
        const list = await loadConversations()
        if (activeConvIdRef.current === id) {
          if (list.length > 0) {
            await switchConversation(list[0].id)
          } else {
            setActiveConvId(null)
            activeConvIdRef.current = null
            setMessages([])
          }
        }
        return true
      } catch (err) {
        console.error('删除对话失败:', err)
        setError('删除对话失败')
        return false
      }
    },
    [loadConversations, switchConversation]
  )

  const renameConversation = useCallback(async (id, title) => {
    if (!title.trim()) return false
    try {
      await RenameConversation(id, title.trim())
      setConversations((prev) =>
        prev.map((c) => (c.id === id ? { ...c, title: title.trim() } : c))
      )
      return true
    } catch (err) {
      console.error('重命名失败:', err)
      setError('重命名失败')
      return false
    }
  }, [])

  const sendMessage = useCallback(
    async (text) => {
      const content = (text ?? inputValue).trim()
      if (!content || isStreaming) return false

      let convId = activeConvIdRef.current
      // 没有活跃对话则先创建一个
      if (!convId) {
        const conv = await CreateConversation('新对话')
        convId = conv.id
        setActiveConvId(convId)
        activeConvIdRef.current = convId
        await loadConversations()
      }

      const userMsg = {
        id: Date.now(),
        conversation_id: convId,
        role: 'user',
        content,
        emotion: '',
        timestamp: new Date(),
      }
      setMessages((prev) => [...prev, userMsg])
      setInputValue('')
      setIsStreaming(true)
      setStreamingContent('')
      streamBufferRef.current = ''
      setError(null)
      // 锁定本条流式归属，切走后仍能正确收尾
      streamingConvIdRef.current = convId

      try {
        await SendMessageStreamInConversation(convId, content)
        return true
      } catch (err) {
        console.error('发送消息失败:', err)
        setIsStreaming(false)
        streamingConvIdRef.current = null
        setError('发送失败，请重试')
        return false
      }
    },
    [inputValue, isStreaming, loadConversations]
  )

  // 流式事件监听：只注册一次，全部用 ref 读取最新状态
  useEffect(() => {
    const handler = (data) => {
      const payload = data || {}
      const { content, done, error: errStr, conversation_id } = payload
      const targetConvId = conversation_id || streamingConvIdRef.current

      // 忽略不属于当前流式归属的事件
      if (
        streamingConvIdRef.current &&
        conversation_id &&
        conversation_id !== streamingConvIdRef.current
      ) {
        return
      }

      if (errStr) {
        setIsStreaming(false)
        streamingConvIdRef.current = null
        setStreamingContent((prev) => `${prev}\n\n[错误] ${errStr}`.trim())
        setError(errStr)
        return
      }

      if (content) {
        streamBufferRef.current += content
        setStreamingContent(streamBufferRef.current)
      }

      if (done) {
        const finalContent = streamBufferRef.current
        const finalConvId = targetConvId || streamingConvIdRef.current
        setIsStreaming(false)
        streamingConvIdRef.current = null
        streamBufferRef.current = ''
        setStreamingContent('')
        if (finalContent) {
          setMessages((prev) => [
            ...prev,
            {
              id: Date.now(),
              conversation_id: finalConvId,
              role: 'assistant',
              content: finalContent,
              emotion: '',
              timestamp: new Date(),
            },
          ])
        }
        // 刷新对话列表（更新时间等）
        loadConversations()
      }
    }

    EventsOn(STREAM_EVENT, handler)
    return () => {
      EventsOff(STREAM_EVENT)
    }
    // 故意空依赖：只注册一次，状态通过 ref / setState 读取
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 初始化：只跑一次
  useEffect(() => {
    let mounted = true
    ;(async () => {
      const list = await loadConversations()
      if (!mounted) return
      if (list.length > 0) {
        await switchConversation(list[0].id)
      }
    })()
    return () => {
      mounted = false
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const clearError = useCallback(() => setError(null), [])

  return {
    // state
    conversations,
    activeConvId,
    messages,
    inputValue,
    isStreaming,
    streamingContent,
    loadingConv,
    error,
    // actions
    setInputValue,
    switchConversation,
    newConversation,
    deleteConversation,
    renameConversation,
    sendMessage,
    loadConversations,
    clearError,
  }
}
