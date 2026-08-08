import React, { useRef, useEffect, useMemo } from 'react'
import { Send, Sparkles, Zap } from 'lucide-react'

// 默认内置斜杠命令（与后端 GetAvailableSlashCommands 的 default 部分保持一致）
const DEFAULT_COMMANDS = [
  { cmd: '/plan', desc: '制定计划 / 设置目标', kind: 'default' },
  { cmd: '/review', desc: '回顾复盘 / 总结', kind: 'default' },
  { cmd: '/memory', desc: '查看记忆 / 回忆', kind: 'default' },
]

// 从当前输入里提取 "/xxx" 前缀（不含空格）
// - 输入 "/"            -> "/"
// - 输入 "/p"           -> "/p"
// - 输入 "/plan"        -> "/plan"
// - 输入 "/plan "       -> ""（已带空格，整体不再视作命令选择态，弹层关闭）
// - 输入 "/plan 学习"   -> ""（同上，已经脱离命令选择态）
// 命中后用这个前缀去匹配命令列表；不命中则视为不在选择态。
function extractCmdPrefix(value) {
  if (!value || !value.startsWith('/')) return ''
  // 找到第一个空格的位置；只要有空格就视为已经脱离选择态
  const spaceIdx = value.indexOf(' ')
  if (spaceIdx !== -1) return ''
  return value
}

// 输入框 + 指令面板
//  - 指令列表 = 内置命令 + 用户在自动化页面创建的自定义命令（由 props.commands 注入）
//  - 列表可上下滚动（max-h + overflow-y-auto）
//  - 逐字母过滤：输入 "/p" 时只保留以 "/p" 开头的指令
//  - 关闭时机：发送消息 / 输入框中命令名后出现空格
function ChatInput({
  value,
  onChange,
  onSend,
  disabled,
  showCommands,
  setShowCommands,
  onCommandSelect,
  commands = [],
}) {
  const textareaRef = useRef(null)

  // 自适应高度
  useEffect(() => {
    const ta = textareaRef.current
    if (!ta) return
    ta.style.height = 'auto'
    ta.style.height = Math.min(ta.scrollHeight, 180) + 'px'
  }, [value])

  // 合并默认 + 自定义命令；自定义同名时覆盖默认（用户主动配置优先）
  const allCommands = useMemo(() => {
    const map = new Map()
    for (const c of DEFAULT_COMMANDS) map.set(c.cmd, c)
    for (const c of commands || []) {
      if (!c || !c.cmd) continue
      map.set(c.cmd, { ...c })
    }
    return Array.from(map.values())
  }, [commands])

  // 计算当前应展示的过滤后指令
  const filteredCommands = useMemo(() => {
    const prefix = extractCmdPrefix(value)
    if (!prefix) return []
    const lower = prefix.toLowerCase()
    return allCommands.filter((c) => c.cmd.toLowerCase().startsWith(lower))
  }, [value, allCommands])

  // 同步"是否显示弹层"：命令前缀有效 + 过滤后还有结果，才显示
  useEffect(() => {
    const shouldShow =
      extractCmdPrefix(value) !== '' && filteredCommands.length > 0
    if (shouldShow !== showCommands) {
      setShowCommands(shouldShow)
    }
    // 依赖 showCommands 可能会触发循环——这里只依赖真正决定开/关的状态
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [value, filteredCommands.length])

  // 触发刷新"自定义命令"的钩子：组件 mount 时拉一次；
  // 父组件可以通过 ref 或 prop 变化刷新，这里留接口给父组件。
  // 实际命令列表由父组件管理（保持单一数据源）。

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      // 发送前主动关闭弹层，避免残余高亮
      setShowCommands(false)
      onSend()
      return
    }
    if (e.key === '/' && value === '') {
      // 用户敲下第一个 "/" 时主动唤起
      e.preventDefault()
      onChange('/')
      setShowCommands(true)
      return
    }
    if (e.key === 'Escape') {
      setShowCommands(false)
    }
    // ↑/↓ 在弹层里切换命令、Tab/Enter 选中第一项等高级交互留作后续
  }

  const handleChange = (e) => {
    const v = e.target.value
    onChange(v)
    // 显式计算：v 是否仍处于"命令选择态"
    const prefix = extractCmdPrefix(v)
    setShowCommands(prefix !== '' && filteredCommandsWillShow(v, allCommands))
  }

  const handleSelect = (cmd) => {
    // 选中后写入"命令 + 空格"，给用户继续输入参数留位置
    onCommandSelect(cmd + ' ')
    setShowCommands(false)
  }

  return (
    <div className="max-w-3xl mx-auto">
      {showCommands && filteredCommands.length > 0 && (
        <div
          className="mb-2 bg-surface border border-border rounded-lg shadow-lg overflow-hidden"
          data-testid="slash-command-popover"
        >
          <div className="flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-text-subtle border-b border-border">
            <Sparkles className="w-3 h-3 text-primary-400" />
            <span>快捷指令</span>
            <span className="text-text-subtle/60 ml-auto">
              {filteredCommands.length} / {allCommands.length}
            </span>
          </div>
          {/* 列表区：max-h + overflow-y-auto 让用户可上下滑动 */}
          <div className="max-h-60 overflow-y-auto p-1">
            {filteredCommands.map((item) => (
              <button
                key={item.cmd}
                onClick={() => handleSelect(item.cmd)}
                className="w-full flex items-center gap-2 px-2 py-2 rounded hover:bg-bg-subtle text-left transition-colors"
              >
                <span className="font-mono text-primary-500 text-sm shrink-0">
                  {item.cmd}
                </span>
                <span className="text-xs text-text-subtle truncate flex-1">
                  {item.desc || ' '}
                </span>
                {item.kind === 'custom' && (
                  <span className="inline-flex items-center gap-1 text-[10px] px-1.5 py-0.5 rounded bg-primary-500/10 text-primary-400 shrink-0">
                    <Zap className="w-2.5 h-2.5" />
                    自定义
                  </span>
                )}
              </button>
            ))}
            {filteredCommands.length === 0 && value && (
              <div className="px-2 py-3 text-xs text-text-subtle text-center">
                没有匹配 {extractCmdPrefix(value)} 的指令
              </div>
            )}
          </div>
        </div>
      )}

      <div className="relative flex items-end gap-2 bg-surface border border-border rounded-xl focus-within:border-primary-400 transition-colors">
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={() => {
            // 重新聚焦时若仍在命令输入态，自动恢复弹层
            if (extractCmdPrefix(value) !== '') setShowCommands(true)
          }}
          placeholder="输入消息，/ 唤起指令菜单，Enter 发送，Shift+Enter 换行..."
          className="flex-1 bg-transparent px-4 py-3 outline-none resize-none text-sm leading-relaxed max-h-44 text-text placeholder:text-text-subtle"
          rows={1}
        />
        <button
          onClick={onSend}
          disabled={!value.trim() || disabled}
          className="m-2 p-2.5 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-40 disabled:cursor-not-allowed flex-shrink-0"
          title="发送"
        >
          <Send size={18} />
        </button>
      </div>
      <div className="mt-2 text-xs text-text-subtle text-center">
        输入 <span className="text-primary-500 font-mono">/</span> 查看快捷指令
      </div>
    </div>
  )
}

// 同步根据 v 和命令列表判断"是否会出现至少一条结果"，
// 避免先 setState 再 useEffect 校正的视觉闪烁。
function filteredCommandsWillShow(v, allCommands) {
  const prefix = extractCmdPrefix(v)
  if (!prefix) return false
  const lower = prefix.toLowerCase()
  return allCommands.some((c) => c.cmd.toLowerCase().startsWith(lower))
}

export default ChatInput
