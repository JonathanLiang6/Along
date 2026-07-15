import React from 'react'
import { Bot, Sparkles } from 'lucide-react'

const SUGGESTED_TOPICS = [
  { icon: '😊', label: '聊聊心情', desc: '情感陪伴', cmd: '今天心情不太好，想找人聊聊' },
  { icon: '💼', label: '工作复盘', desc: '工作相关', cmd: '/review 本周工作' },
  { icon: '📚', label: '学习计划', desc: '学习规划', cmd: '/plan 学习计划' },
  { icon: '🤔', label: '技术探索', desc: '技术发现', cmd: '什么是最新的AI技术？' },
]

// 空状态：欢迎页 + 建议话题
function WelcomeScreen({ onPickTopic }) {
  return (
    <div className="flex flex-col items-center justify-center min-h-[60vh] text-center px-4">
      <div className="w-14 h-14 rounded-full overflow-hidden mb-3 shadow-lg">
        <img
          src="/logo.png"
          alt="Along"
          className="w-full h-full object-cover"
          onError={(e) => { e.target.style.display = 'none' }}
        />
      </div>
      <h2 className="text-xl font-bold mb-1.5 text-text">你好</h2>
      <p className="text-text-muted mb-5 max-w-md text-sm">
        我是你的 AI 陪伴伙伴，可以陪你聊天、制定计划、帮你回忆事情，还能做总结复盘。
      </p>
      <div className="grid grid-cols-2 gap-2 w-full max-w-lg">
        {SUGGESTED_TOPICS.map((topic, i) => (
          <button
            key={i}
            onClick={() => onPickTopic(topic.cmd)}
            className="px-3 py-2.5 rounded-xl border border-border hover:border-primary-400 hover:bg-primary-500/5 transition-all group bg-surface flex items-center gap-3"
          >
            <span className="text-2xl shrink-0">{topic.icon}</span>
            <div className="min-w-0 text-left">
              <div className="font-medium text-text group-hover:text-primary-500 transition-colors">
                {topic.label}
              </div>
              <div className="text-xs text-text-muted">{topic.desc}</div>
            </div>
          </button>
        ))}
      </div>
      <div className="mt-5 flex items-center gap-1.5 text-xs text-text-subtle">
        <Sparkles size={14} className="text-primary-400" />
        选择话题或在下方输入消息开始对话
      </div>
    </div>
  )
}

export default WelcomeScreen
