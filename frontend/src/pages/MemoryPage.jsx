import React, { useState, useEffect, useCallback } from 'react'
import { Brain, HeartHandshake, Star, Calendar, Award, Tag, Edit2, Trash2, Save, X, Loader2, AlertCircle, Search, Plus, Sparkles } from 'lucide-react'

// PRD 5: 5层记忆体系
// L1 个人画像 / L2 情感关系 / L3 关键事件 / L4 项目目标 / L5 日常喜好
const memoryTypes = [
  { id: '', label: '全部', short: '全部', icon: Brain, color: 'text-text-muted', description: '所有记忆' },
  { id: 'L1', label: 'L1 个人画像', short: '画像', icon: Tag, color: 'text-blue-400', description: '名字、年龄、职业' },
  { id: 'L2', label: 'L2 情感关系', short: '关系', icon: HeartHandshake, color: 'text-purple-400', description: '家人、朋友、伴侣' },
  { id: 'L3', label: 'L3 关键事件', short: '事件', icon: Star, color: 'text-amber-400', description: '重要日期、转折点' },
  { id: 'L4', label: 'L4 计划目标', short: '计划', icon: Award, color: 'text-accent-400', description: '正在做的事' },
  { id: 'L5', label: 'L5 日常喜好', short: '喜好', icon: Calendar, color: 'text-green-400', description: '口味、爱好、习惯' },
]

// 格式化后端 time.Time 为 YYYY-MM-DD
const formatDate = (raw) => {
  if (!raw) return ''
  const str = typeof raw === 'string' ? raw : String(raw)
  // 兼容 ISO/RFC3339 / 普通日期字符串
  const datePart = str.split('T')[0]
  if (/^\d{4}-\d{2}-\d{2}$/.test(datePart)) return datePart
  const d = new Date(str)
  if (isNaN(d.getTime())) return str
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, '0')
  const day = String(d.getDate()).padStart(2, '0')
  return `${y}-${m}-${day}`
}

// 判断 window.go 是否可用
const hasBackend = () => {
  try {
    return typeof window !== 'undefined' && window.go && window.go.main && window.go.main.App
  } catch (e) {
    return false
  }
}

function MemoryPage() {
  const [activeType, setActiveType] = useState('')
  const [memories, setMemories] = useState([])
  const [counts, setCounts] = useState({ L1: 0, L2: 0, L3: 0, L4: 0, L5: 0 })
  const [editingId, setEditingId] = useState(null)
  const [editContent, setEditContent] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState(null)
  const [actionLoading, setActionLoading] = useState(null)
  const [searchKeyword, setSearchKeyword] = useState('')
  const [showAddDialog, setShowAddDialog] = useState(false)
  const [newMemory, setNewMemory] = useState({ type: 'L1', content: '' })

  const backendAvailable = hasBackend()

  const loadMemories = useCallback(async (typeId, keyword = '') => {
    if (!hasBackend()) {
      setMemories([])
      return
    }
    setLoading(true)
    setError(null)
    try {
      let result
      if (keyword && keyword.trim()) {
        // 搜索模式
        result = await window.go.main.App.GlobalSearch(keyword)
        // 兼容不同的返回结构
        if (Array.isArray(result)) {
          result = { memories: result }
        }
        // 提取 memories 部分
        let allMemories = result.memories || []
        if (typeId) {
          allMemories = allMemories.filter((m) => m.type === typeId)
        }
        setMemories(allMemories)
      } else {
        result = await window.go.main.App.GetMemories(typeId || '')
        const list = Array.isArray(result) ? result : []
        setMemories(list)
      }
    } catch (err) {
      console.error('GetMemories failed:', err)
      setError(err?.message || String(err) || '获取记忆失败')
      setMemories([])
    } finally {
      setLoading(false)
    }
  }, [])

  const loadCounts = useCallback(async () => {
    if (!hasBackend()) return
    try {
      const result = await window.go.main.App.GetMemoryCountByType()
      if (result && typeof result === 'object') {
        setCounts({
          L1: result.L1 || 0,
          L2: result.L2 || 0,
          L3: result.L3 || 0,
          L4: result.L4 || 0,
          L5: result.L5 || 0,
        })
      }
    } catch (err) {
      console.error('GetMemoryCountByType failed:', err)
    }
  }, [])

  // 组件加载时获取全部记忆与各类型数量
  useEffect(() => {
    loadMemories('')
    loadCounts()
  }, [loadMemories, loadCounts])

  // 监听操作完成后的刷新
  useEffect(() => {
    if (actionLoading === null) {
      loadCounts()
    }
  }, [actionLoading, loadCounts])

  const handleTypeChange = (typeId) => {
    setActiveType(typeId)
    loadMemories(typeId, searchKeyword)
  }

  const handleSearch = (e) => {
    e.preventDefault()
    loadMemories(activeType, searchKeyword)
  }

  const handleClearSearch = () => {
    setSearchKeyword('')
    loadMemories(activeType, '')
  }

  const handleEdit = (memory) => {
    setEditingId(memory.id)
    setEditContent(memory.content || '')
  }

  const handleSave = async () => {
    if (editingId == null) return
    setActionLoading('update')
    setError(null)
    try {
      await window.go.main.App.UpdateMemory(editingId, editContent)
      setEditingId(null)
      await loadMemories(activeType, searchKeyword)
      await loadCounts()
    } catch (err) {
      console.error('UpdateMemory failed:', err)
      setError(err?.message || String(err) || '更新记忆失败')
    } finally {
      setActionLoading(null)
    }
  }

  const handleDelete = async (id) => {
    if (!confirm('确定要删除这条记忆吗？')) return
    setActionLoading(id)
    setError(null)
    try {
      await window.go.main.App.DeleteMemory(id)
      await loadMemories(activeType, searchKeyword)
      await loadCounts()
    } catch (err) {
      console.error('DeleteMemory failed:', err)
      setError(err?.message || String(err) || '删除记忆失败')
    } finally {
      setActionLoading(null)
    }
  }

  const handleAddMemory = async () => {
    if (!newMemory.content.trim()) {
      setError('请输入记忆内容')
      return
    }
    setActionLoading('add')
    setError(null)
    try {
      await window.go.main.App.AddMemory(newMemory.type, newMemory.content.trim(), 'manual', 1.0)
      setShowAddDialog(false)
      setNewMemory({ type: 'L1', content: '' })
      await loadMemories(activeType, searchKeyword)
      await loadCounts()
    } catch (err) {
      console.error('AddMemory failed:', err)
      setError(err?.message || String(err) || '添加失败')
    } finally {
      setActionLoading(null)
    }
  }

  const getTypeLabel = (typeId) => {
    return memoryTypes.find((t) => t.id === typeId)?.label || typeId
  }

  const getTypeColor = (typeId) => {
    return memoryTypes.find((t) => t.id === typeId)?.color || 'text-text-muted'
  }

  const getCount = (typeId) => {
    if (typeId === '') {
      return counts.L1 + counts.L2 + counts.L3 + counts.L4 + counts.L5
    }
    return counts[typeId] || 0
  }

  if (!backendAvailable) {
    return (
      <div className="max-w-3xl mx-auto py-4 px-4">
        <h2 className="text-lg font-semibold text-text mb-4">记忆</h2>
        <div className="text-center py-12 text-text-subtle">
          <Brain className="w-8 h-8 mx-auto mb-3 opacity-50" />
          <p className="text-sm">后端服务未连接</p>
          <p className="text-xs mt-1">请确保应用以 Wails 桌面模式运行</p>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-3xl mx-auto py-4 px-4">
      <div className="flex items-center justify-between mb-4">
        <div>
          <h2 className="text-lg font-semibold text-text">记忆</h2>
          <p className="text-xs text-text-subtle mt-0.5">5层记忆体系 · 共 {getCount('')} 条</p>
        </div>
        <button
          onClick={() => setShowAddDialog(true)}
          className="px-3 py-1.5 bg-primary-600 hover:bg-primary-500 text-white text-sm rounded-lg transition-colors flex items-center gap-1.5"
        >
          <Plus className="w-3.5 h-3.5" />
          新增记忆
        </button>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="mb-4 flex items-start gap-2 bg-red-500/10 border border-red-500/30 text-red-400 rounded-lg px-3 py-2 text-sm">
          <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <div className="flex-1">
            <p>{error}</p>
          </div>
          <button onClick={() => setError(null)} className="p-0.5 hover:text-red-300 transition-colors">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* 搜索 */}
      <form onSubmit={handleSearch} className="mb-4">
        <div className="relative">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-text-subtle" />
          <input
            type="text"
            value={searchKeyword}
            onChange={(e) => setSearchKeyword(e.target.value)}
            placeholder="搜索记忆内容..."
            className="w-full bg-surface-subtle border border-border rounded-lg pl-9 pr-9 py-2 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20"
          />
          {searchKeyword && (
            <button
              type="button"
              onClick={handleClearSearch}
              className="absolute right-2 top-1/2 -translate-y-1/2 p-1 text-text-subtle hover:text-text-muted"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          )}
        </div>
      </form>

      {/* 记忆类型筛选（显示真实总数） */}
      <div className="flex gap-2 mb-4 overflow-x-auto pb-1">
        {memoryTypes.map((type) => {
          const Icon = type.icon
          const isActive = activeType === type.id
          const count = getCount(type.id)
          return (
            <button
              key={type.id}
              onClick={() => handleTypeChange(type.id)}
              disabled={loading}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm whitespace-nowrap transition-colors flex-shrink-0 ${
                isActive
                  ? 'bg-bg-hover text-text'
                  : 'text-text-subtle hover:text-text-muted hover:bg-surface-subtle/50'
              } ${loading ? 'opacity-50 cursor-not-allowed' : ''}`}
              title={type.description}
            >
              <Icon className={`w-4 h-4 ${isActive ? type.color : ''}`} />
              <span>{type.label}</span>
              <span className={`text-xs ${isActive ? 'text-text-muted' : 'text-text-subtle'}`}>({count})</span>
            </button>
          )
        })}
      </div>

      {/* 加载状态 */}
      {loading ? (
        <div className="flex items-center justify-center py-12 text-text-subtle">
          <Loader2 className="w-5 h-5 animate-spin mr-2" />
          <span className="text-sm">加载中...</span>
        </div>
      ) : (
        <>
          {/* 记忆列表 */}
          <div className="space-y-2">
            {memories.map((memory) => (
              <div
                key={memory.id}
                className="bg-surface/60 border border-border rounded-xl p-4 hover:border-border-strong transition-colors group"
              >
                {editingId === memory.id ? (
                  <div className="space-y-2">
                    <textarea
                      value={editContent}
                      onChange={(e) => setEditContent(e.target.value)}
                      rows={2}
                      className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-text focus:outline-none focus:border-primary-500/50 resize-none"
                    />
                    <div className="flex gap-2 justify-end">
                      <button
                        onClick={() => setEditingId(null)}
                        disabled={actionLoading === 'update'}
                        className="p-1.5 text-text-subtle hover:text-text-muted transition-colors disabled:opacity-50"
                      >
                        <X className="w-4 h-4" />
                      </button>
                      <button
                        onClick={handleSave}
                        disabled={actionLoading === 'update'}
                        className="p-1.5 text-primary-400 hover:text-primary-300 transition-colors disabled:opacity-50"
                      >
                        {actionLoading === 'update' ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Save className="w-4 h-4" />
                        )}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-2 mb-1.5">
                        <span className={`text-xs font-medium ${getTypeColor(memory.type)}`}>
                          {getTypeLabel(memory.type)}
                        </span>
                        <div className="flex items-center gap-1">
                          <div className="w-8 h-1 bg-surface rounded-full overflow-hidden">
                            <div
                              className="h-full bg-primary-500 rounded-full"
                              style={{ width: `${Math.max(0, Math.min(1, memory.confidence || 0)) * 100}%` }}
                            />
                          </div>
                          <span className="text-xs text-text-subtle">
                            {Math.round((memory.confidence || 0) * 100)}%
                          </span>
                        </div>
                      </div>
                      <p className="text-sm text-text">{memory.content}</p>
                      <div className="text-xs text-text-subtle mt-1.5">
                        {formatDate(memory.created_at || memory.createdAt)}
                      </div>
                    </div>
                    <div className="flex gap-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={() => handleEdit(memory)}
                        disabled={actionLoading === memory.id}
                        className="p-1.5 text-text-subtle hover:text-primary-400 transition-colors disabled:opacity-50"
                      >
                        <Edit2 className="w-4 h-4" />
                      </button>
                      <button
                        onClick={() => handleDelete(memory.id)}
                        disabled={actionLoading === memory.id}
                        className="p-1.5 text-text-subtle hover:text-red-400 transition-colors disabled:opacity-50"
                      >
                        {actionLoading === memory.id ? (
                          <Loader2 className="w-4 h-4 animate-spin" />
                        ) : (
                          <Trash2 className="w-4 h-4" />
                        )}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>

          {memories.length === 0 && (
            <div className="text-center py-12 text-text-subtle">
              <Brain className="w-8 h-8 mx-auto mb-3 opacity-50" />
              <p className="text-sm">
                {searchKeyword ? `没有找到包含「${searchKeyword}」的记忆` : '还没有这类记忆'}
              </p>
              <p className="text-xs mt-1">多聊聊天，记忆会慢慢积累</p>
            </div>
          )}
        </>
      )}

      {/* 新增记忆对话框 */}
      {showAddDialog && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowAddDialog(false)}>
          <div
            className="bg-surface-subtle border border-border rounded-xl p-6 w-full max-w-md mx-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-base font-semibold text-text flex items-center gap-2">
                <Sparkles className="w-4 h-4 text-primary-400" />
                新增记忆
              </h3>
              <button onClick={() => setShowAddDialog(false)} className="p-1 text-text-subtle hover:text-text-muted">
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-3">
              <div>
                <label className="block text-xs text-text-muted mb-1.5">记忆类型</label>
                <div className="grid grid-cols-5 gap-1.5">
                  {memoryTypes.filter((t) => t.id !== '').map((type) => {
                    const Icon = type.icon
                    const isActive = newMemory.type === type.id
                    return (
                      <button
                        key={type.id}
                        onClick={() => setNewMemory({ ...newMemory, type: type.id })}
                        className={`flex flex-col items-center gap-1 p-2 rounded-lg text-xs transition-colors ${
                          isActive
                            ? 'bg-bg-hover text-text'
                            : 'bg-surface text-text-subtle hover:text-text-muted'
                        }`}
                      >
                        <Icon className={`w-4 h-4 ${isActive ? type.color : ''}`} />
                        <span>{type.short}</span>
                      </button>
                    )
                  })}
                </div>
              </div>
              <div>
                <label className="block text-xs text-text-muted mb-1.5">内容</label>
                <textarea
                  value={newMemory.content}
                  onChange={(e) => setNewMemory({ ...newMemory, content: e.target.value })}
                  rows={3}
                  placeholder="例如：用户喜欢看科幻电影，尤其是星际穿越"
                  className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20 resize-none"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-5">
              <button
                onClick={() => setShowAddDialog(false)}
                disabled={actionLoading === 'add'}
                className="px-3 py-1.5 text-text-muted hover:text-text text-sm transition-colors"
              >
                取消
              </button>
              <button
                onClick={handleAddMemory}
                disabled={actionLoading === 'add' || !newMemory.content.trim()}
                className="px-4 py-1.5 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white text-sm rounded-lg transition-colors flex items-center gap-1.5"
              >
                {actionLoading === 'add' ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Save className="w-3.5 h-3.5" />
                )}
                保存
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default MemoryPage
