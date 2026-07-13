import React from 'react'
import { Bot, Sparkles } from 'lucide-react'

const SUGGESTED_TOPICS = [
  { icon: '📝', label: '帮我制定一个学习计划', cmd: '/plan 学习计划' },
  { icon: '🔍', label: '最近有什么新鲜事', cmd: '最近有什么新鲜事？' },
  { icon: '💭', label: '回顾一下这周的成长', cmd: '/review 本周' },
  { icon: '😊', label: '今天心情不太好', cmd: '今天心情不太好，想找人聊聊' },
]

// 空状态：欢迎页 + 建议话题
function WelcomeScreen({ onPickTopic }) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center">
      <div className="w-16 h-16 rounded-full overflow-hidden mb-4 shadow-lg">
        <img
          src="./src/assets/logo-icon.png"
          alt="Along"
          className="w-full h-full object-cover"
          onError={(e) => { e.target.style.display = 'none' }}
        />
      </div>
      <h2 className="text-2xl font-bold mb-2 text-text">你好</h2>
      <p className="text-text-muted mb-8 max-w-md">
        我是你的 AI 陪伴伙伴，可以陪你聊天、制定计划、帮你回忆事情，还能做总结复盘。
      </p>
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 w-full max-w-lg">
        {SUGGESTED_TOPICS.map((topic, i) => (
          <button
            key={i}
            onClick={() => onPickTopic(topic.cmd)}
            className="p-4 text-left rounded-xl border border-border hover:border-primary-400 hover:bg-primary-500/5 transition-all group bg-surface"
          >
            <div className="text-xl mb-1">{topic.icon}</div>
            <div className="text-sm font-medium text-text group-hover:text-primary-500 transition-colors">
              {topic.label}
            </div>
          </button>
        ))}
      </div>
      <div className="mt-8 flex items-center gap-1.5 text-xs text-text-subtle">
        <Sparkles size={14} className="text-primary-400" />
        选择话题或在下方输入消息开始对话
      </div>
    </div>
  )
}

export default WelcomeScreen
