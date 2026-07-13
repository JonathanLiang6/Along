import React, { useRef, useEffect } from 'react'
import { Send } from 'lucide-react'

const COMMANDS = [
  { cmd: '/plan', desc: '制定计划 / 设置目标' },
  { cmd: '/review', desc: '回顾复盘 / 总结' },
  { cmd: '/memory', desc: '查看记忆 / 回忆' },
]

// 输入框 + 指令面板
function ChatInput({
  value,
  onChange,
  onSend,
  disabled,
  showCommands,
  setShowCommands,
  onCommandSelect,
}) {
  const textareaRef = useRef(null)

  // 自适应高度
  useEffect(() => {
    const ta = textareaRef.current
    if (!ta) return
    ta.style.height = 'auto'
    ta.style.height = Math.min(ta.scrollHeight, 180) + 'px'
  }, [value])

  const handleKeyDown = (e) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      onSend()
      return
    }
    if (e.key === '/' && value === '') {
      e.preventDefault()
      setShowCommands(true)
      onChange('/')
      return
    }
    if (e.key === 'Escape') {
      setShowCommands(false)
    }
  }

  const handleChange = (e) => {
    const v = e.target.value
    onChange(v)
    setShowCommands(v.startsWith('/'))
  }

  return (
    <div className="max-w-3xl mx-auto">
      {showCommands && (
        <div className="mb-2 p-2 bg-surface border border-border rounded-lg shadow-lg">
          <div className="text-xs text-text-subtle px-2 py-1">快捷指令</div>
          {COMMANDS.map((item) => (
            <button
              key={item.cmd}
              onClick={() => onCommandSelect(item.cmd)}
              className="w-full flex items-center justify-between px-2 py-2 rounded hover:bg-bg-subtle text-left"
            >
              <span className="font-mono text-primary-500 text-sm">
                {item.cmd}
              </span>
              <span className="text-xs text-text-subtle">{item.desc}</span>
            </button>
          ))}
        </div>
      )}

      <div className="relative flex items-end gap-2 bg-surface border border-border rounded-xl focus-within:border-primary-400 transition-colors">
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          onFocus={() => value.startsWith('/') && setShowCommands(true)}
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

export default ChatInput
