import React, { useState } from 'react'
import {
  Plus,
  Trash2,
  Edit3,
  Check,
  X,
  MessageSquare,
  Loader2,
  Bot,
} from 'lucide-react'

// 侧边栏：对话列表 + 新建 + 重命名/删除
function ChatSidebar({
  conversations,
  activeConvId,
  isStreaming,
  loading,
  onNew,
  onSelect,
  onRename,
  onDelete,
}) {
  const [editingId, setEditingId] = useState(null)
  const [editingTitle, setEditingTitle] = useState('')

  const startRename = (conv, e) => {
    e?.stopPropagation()
    setEditingId(conv.id)
    setEditingTitle(conv.title || '')
  }

  const cancelRename = (e) => {
    e?.stopPropagation()
    setEditingId(null)
    setEditingTitle('')
  }

  const confirmRename = async (id, e) => {
    e?.stopPropagation()
    const ok = await onRename(id, editingTitle)
    if (ok) {
      setEditingId(null)
      setEditingTitle('')
    }
  }

  const handleDelete = (conv, e) => {
    e?.stopPropagation()
    if (!window.confirm('确定要删除这个对话吗？')) return
    onDelete(conv.id)
  }

  return (
    <div className="flex flex-col h-full">
      {/* 顶部：Logo + 新建对话 */}
      <div className="p-3 border-b border-border">
        <div className="flex items-center justify-center mb-3">
          <img
            src="./src/assets/logo.png"
            alt="Along"
            className="w-8 h-8 rounded-full object-cover"
            onError={(e) => { e.target.style.display = 'none' }}
          />
        </div>
        <button
          onClick={onNew}
          disabled={isStreaming}
          className="w-full flex items-center justify-center gap-2 px-4 py-2.5 bg-primary-500 text-white rounded-lg hover:bg-primary-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed font-medium"
        >
          <Plus size={18} />
          新建对话
        </button>
      </div>

      {/* 列表 */}
      <div className="flex-1 overflow-y-auto py-2">
        {loading && conversations.length === 0 ? (
          <div className="flex items-center justify-center text-text-subtle text-sm py-8 gap-2">
            <Loader2 size={14} className="animate-spin" />
            加载中...
          </div>
        ) : conversations.length === 0 ? (
          <div className="text-center text-text-subtle text-sm py-8 px-4">
            暂无对话，点击上方按钮开始
          </div>
        ) : (
          <div className="space-y-0.5 px-2">
            {conversations.map((conv) => (
              <div
                key={conv.id}
                onClick={() => onSelect(conv.id)}
                className={`group relative flex items-center gap-2 px-3 py-2.5 rounded-lg cursor-pointer transition-colors ${
                  activeConvId === conv.id
                    ? 'bg-primary-500/15 text-primary-600 dark:text-primary-300'
                    : 'hover:bg-bg-subtle text-text'
                }`}
              >
                <MessageSquare
                  size={16}
                  className="flex-shrink-0 opacity-70"
                />
                {editingId === conv.id ? (
                  <div className="flex-1 flex items-center gap-1">
                    <input
                      type="text"
                      value={editingTitle}
                      onChange={(e) => setEditingTitle(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') confirmRename(conv.id, e)
                        if (e.key === 'Escape') cancelRename(e)
                      }}
                      className="flex-1 bg-transparent border-b border-primary-400 outline-none text-sm min-w-0"
                      autoFocus
                      onClick={(e) => e.stopPropagation()}
                    />
                    <button
                      onClick={(e) => confirmRename(conv.id, e)}
                      className="p-1 text-success-500 hover:bg-success-500/20 rounded"
                      title="确认"
                    >
                      <Check size={14} />
                    </button>
                    <button
                      onClick={cancelRename}
                      className="p-1 text-danger-500 hover:bg-danger-500/20 rounded"
                      title="取消"
                    >
                      <X size={14} />
                    </button>
                  </div>
                ) : (
                  <>
                    <span className="flex-1 text-sm truncate">
                      {conv.title || '未命名对话'}
                    </span>
                    <div className="flex-shrink-0 flex items-center gap-0.5 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={(e) => startRename(conv, e)}
                        className="p-1 hover:bg-bg-subtle rounded text-text-subtle hover:text-text"
                        title="重命名"
                      >
                        <Edit3 size={13} />
                      </button>
                      <button
                        onClick={(e) => handleDelete(conv, e)}
                        className="p-1 hover:bg-danger-500/20 rounded text-text-subtle hover:text-danger-400"
                        title="删除"
                      >
                        <Trash2 size={13} />
                      </button>
                    </div>
                  </>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

export default ChatSidebar
