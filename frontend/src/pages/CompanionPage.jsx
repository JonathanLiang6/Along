import React, { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { Menu, ChevronLeft, Sparkles, ChevronDown, AlertCircle, X } from 'lucide-react'
import { useChat, STREAMING_MSG_ID } from '../hooks/useChat'
import ChatSidebar from '../components/chat/ChatSidebar'
import MessageBubble from '../components/chat/MessageBubble'
import ChatInput from '../components/chat/ChatInput'
import WelcomeScreen from '../components/chat/WelcomeScreen'

// 距底部多少像素内算“在底部”，超过则不自动滚（避免打断阅读）
const NEAR_BOTTOM_THRESHOLD = 80

export default function CompanionPage() {
  const chat = useChat()
  const {
    conversations,
    activeConvId,
    messages,
    inputValue,
    isStreaming,
    streamingContent,
    loadingConv,
    error,
    setInputValue,
    switchConversation,
    newConversation,
    deleteConversation,
    renameConversation,
    sendMessage,
    clearError,
  } = chat

  const [sidebarOpen, setSidebarOpen] = useState(true)
  const [showCommands, setShowCommands] = useState(false)
  const [isAtBottom, setIsAtBottom] = useState(true)

  const scrollRef = useRef(null)
  const bottomRef = useRef(null)

  // 展示用消息列表：流式中追加一条临时流式消息
  const displayMessages = useMemo(() => {
    if (isStreaming && streamingContent) {
      return [
        ...messages,
        {
          id: STREAMING_MSG_ID,
          role: 'assistant',
          content: streamingContent,
        },
      ]
    }
    return messages
  }, [messages, isStreaming, streamingContent])

  // 当前对话标题
  const activeTitle = useMemo(() => {
    if (!activeConvId) return '新对话'
    const conv = conversations.find((c) => c.id === activeConvId)
    return conv?.title || '对话'
  }, [activeConvId, conversations])

  // 滚动到底部
  const scrollToBottom = useCallback((behavior = 'smooth') => {
    bottomRef.current?.scrollIntoView({ behavior, block: 'end' })
  }, [])

  // 检测是否在底部附近
  const handleScroll = useCallback((e) => {
    const el = e.target
    if (!el) return
    const distance = el.scrollHeight - el.scrollTop - el.clientHeight
    setIsAtBottom(distance < NEAR_BOTTOM_THRESHOLD)
  }, [])

  // 新消息或流式更新时，若用户在底部则自动滚
  useEffect(() => {
    if (isAtBottom) {
      scrollToBottom(isStreaming ? 'auto' : 'smooth')
    }
  }, [displayMessages, isAtBottom, isStreaming, scrollToBottom])

  // 切换对话时立即贴底（无动画）
  useEffect(() => {
    setIsAtBottom(true)
    scrollToBottom('auto')
  }, [activeConvId, scrollToBottom])

  const handleCommandSelect = (cmd) => {
    setInputValue(cmd + ' ')
    setShowCommands(false)
  }

  const handlePickTopic = (cmd) => {
    setInputValue(cmd)
  }

  return (
    <div className="flex h-full w-full bg-bg text-text overflow-hidden">
      {/* 侧边栏 */}
      <div
        className={`flex-shrink-0 transition-all duration-300 overflow-hidden border-r border-border bg-surface ${
          sidebarOpen ? 'w-64' : 'w-0'
        }`}
      >
        <ChatSidebar
          conversations={conversations}
          activeConvId={activeConvId}
          isStreaming={isStreaming}
          loading={loadingConv}
          onNew={newConversation}
          onSelect={switchConversation}
          onRename={renameConversation}
          onDelete={deleteConversation}
        />
      </div>

      {/* 主聊天区域 */}
      <div className="flex-1 flex flex-col overflow-hidden min-w-0">
        {/* 顶部栏 */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border bg-bg">
          <button
            onClick={() => setSidebarOpen((v) => !v)}
            className="p-2 hover:bg-bg-subtle rounded-lg transition-colors text-text-muted hover:text-text"
            title={sidebarOpen ? '收起侧边栏' : '展开侧边栏'}
          >
            {sidebarOpen ? <ChevronLeft size={18} /> : <Menu size={18} />}
          </button>
          <div className="flex-1" />
          <div className="text-xs text-text-subtle flex items-center gap-1 flex-shrink-0">
            <Sparkles size={14} className="text-primary-400" />
            AI 陪伴
          </div>
        </div>

        {/* 错误条 */}
        {error && (
          <div className="flex items-center gap-2 px-4 py-2 bg-danger-500/10 border-b border-danger-500/20 text-danger-500 text-sm">
            <AlertCircle size={14} className="flex-shrink-0" />
            <span className="flex-1 truncate">{error}</span>
            <button
              onClick={clearError}
              className="p-0.5 hover:bg-danger-500/20 rounded"
              title="关闭"
            >
              <X size={14} />
            </button>
          </div>
        )}

        {/* 消息列表 */}
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="flex-1 overflow-y-auto relative"
        >
          <div className="max-w-3xl mx-auto px-4 py-6">
            {displayMessages.length === 0 ? (
              <WelcomeScreen onPickTopic={handlePickTopic} />
            ) : (
              <>
                <div className="space-y-6">
                  {displayMessages.map((msg) => (
                    <MessageBubble
                      key={msg.id || msg.role + '-' + (msg.timestamp || '')}
                      msg={msg}
                      isStreamingMsg={msg.id === STREAMING_MSG_ID}
                    />
                  ))}
                  <div ref={bottomRef} />
                </div>

                {/* 滚动到底部按钮 */}
                {!isAtBottom && (
                  <button
                    onClick={() => {
                      setIsAtBottom(true)
                      scrollToBottom('smooth')
                    }}
                    className="fixed bottom-32 right-8 w-10 h-10 rounded-full bg-surface border border-border shadow-lg flex items-center justify-center text-text-muted hover:text-text hover:border-primary-400 transition-colors"
                    title="滚动到底部"
                  >
                    <ChevronDown size={18} />
                  </button>
                )}
              </>
            )}
          </div>
        </div>

        {/* 输入区域 */}
        <div className="border-t border-border bg-bg p-4">
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={() => sendMessage()}
            disabled={isStreaming}
            showCommands={showCommands}
            setShowCommands={setShowCommands}
            onCommandSelect={handleCommandSelect}
          />
        </div>
      </div>
    </div>
  )
}
