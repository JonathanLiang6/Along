import React, { useState, useEffect, useCallback } from 'react'
import {
  Plus,
  ChevronLeft,
  BookOpen,
  Rocket,
  Leaf,
  Sparkles,
  Check,
  Calendar,
  Edit3,
  Trash2,
  Target,
  Clock,
  MessageCircle,
  X,
  ChevronRight,
  PlusCircle,
} from 'lucide-react'

const hasBackend = () => {
  try {
    return typeof window !== 'undefined' && window.go && window.go.main && window.go.main.App
  } catch (e) {
    return false
  }
}

const PLAN_TYPES = [
  { id: 'learning', label: '学习', icon: BookOpen, color: 'text-primary-400', bg: 'bg-primary-500/10' },
  { id: 'project', label: '项目', icon: Rocket, color: 'text-accent-400', bg: 'bg-accent-500/10' },
  { id: 'habit', label: '习惯', icon: Leaf, color: 'text-success-400', bg: 'bg-success-500/10' },
  { id: 'life', label: '生活', icon: Sparkles, color: 'text-warning-400', bg: 'bg-warning-500/10' },
]

const STATUS_LABEL = {
  active: '进行中',
  completed: '已完成',
  paused: '暂停中',
  dropped: '已放弃',
}

function getTypeInfo(type) {
  return PLAN_TYPES.find((t) => t.id === type) || PLAN_TYPES[1]
}

function formatDate(d) {
  if (!d) return ''
  const date = new Date(d)
  if (isNaN(date.getTime())) return d
  const m = (date.getMonth() + 1).toString().padStart(2, '0')
  const day = date.getDate().toString().padStart(2, '0')
  return `${date.getFullYear()}.${m}.${day}`
}

function PlanCard({ goal, onClick }) {
  const typeInfo = getTypeInfo(goal.type)
  const TypeIcon = typeInfo.icon

  return (
    <div
      onClick={onClick}
      className="group bg-surface/60 border border-border/80 rounded-xl p-4 cursor-pointer
                 hover:border-primary-500/40 hover:bg-surface-subtle transition-all duration-200
                 hover:shadow-lg hover:shadow-primary-500/5"
    >
      <div className="flex items-start justify-between mb-3">
        <div className="flex items-center gap-2">
          <div className={`w-8 h-8 rounded-lg ${typeInfo.bg} flex items-center justify-center`}>
            <TypeIcon className={`w-4 h-4 ${typeInfo.color}`} />
          </div>
          <div>
            <h3 className="font-medium text-text text-sm leading-tight">{goal.title}</h3>
            <div className="flex items-center gap-2 mt-0.5">
              <span className="text-xs text-text-subtle">{typeInfo.label}</span>
              <span className="text-text-subtle">·</span>
              <span className={`text-xs ${
            goal.status === 'active' ? 'text-success-400' :
            goal.status === 'completed' ? 'text-primary-400' :
            goal.status === 'paused' ? 'text-warning-400' : 'text-text-subtle'
          }`}>
            {STATUS_LABEL[goal.status] || goal.status}
          </span>
            </div>
          </div>
        </div>
        <ChevronRight className="w-4 h-4 text-text-subtle group-hover:text-primary-400 transition-colors" />
      </div>

      {goal.description && (
        <p className="text-xs text-text-muted line-clamp-2 mb-3">{goal.description}</p>
      )}

      <div className="mb-3">
        <div className="flex items-center justify-between mb-1.5">
          <span className="text-xs text-text-subtle">进度</span>
          <span className="text-xs text-text-muted font-medium">{goal.progress || 0}%</span>
        </div>
        <div className="h-1.5 bg-bg-hover rounded-full overflow-hidden">
          <div
            className="h-full bg-gradient-to-r from-primary-500 to-accent-400 rounded-full transition-all duration-500"
            style={{ width: `${goal.progress || 0}%` }}
          />
        </div>
      </div>

      {goal.companion_note && (
        <div className="flex items-start gap-2 p-2.5 rounded-lg bg-surface/60 border border-border/50">
          <MessageCircle className="w-3.5 h-3.5 text-primary-400 mt-0.5 flex-shrink-0" />
          <p className="text-xs text-text-muted leading-relaxed line-clamp-2">{goal.companion_note}</p>
        </div>
      )}
    </div>
  )
}

function CreatePlanModal({ onClose, onCreate }) {
  const [title, setTitle] = useState('')
  const [description, setDescription] = useState('')
  const [type, setType] = useState('project')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e) => {
    e.preventDefault()
    if (!title.trim()) return
    setLoading(true)
    try {
      await onCreate(title.trim(), description.trim(), type)
      onClose()
    } catch (err) {
      console.error('创建失败:', err)
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 bg-black/60 flex items-end sm:items-center justify-center" onClick={onClose}>
      <div
        className="w-full sm:max-w-md bg-surface border border-border rounded-t-2xl sm:rounded-2xl shadow-2xl animate-slide-up"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <h2 className="font-semibold text-text">新的计划</h2>
          <button onClick={onClose} className="p-1 rounded-lg hover:bg-surface-subtle text-text-muted hover:text-text">
            <X className="w-5 h-5" />
          </button>
        </div>

        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          <div>
            <label className="text-xs text-text-muted mb-1.5 block">计划名称</label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="你想做什么？"
              autoFocus
              className="w-full bg-surface-subtle border border-border rounded-lg px-3 py-2.5 text-sm
                         text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50
                         transition-colors"
            />
          </div>

          <div>
            <label className="text-xs text-text-muted mb-1.5 block">类型</label>
            <div className="grid grid-cols-4 gap-2">
              {PLAN_TYPES.map((t) => {
                const Icon = t.icon
                const active = type === t.id
                return (
                  <button
                    key={t.id}
                    type="button"
                    onClick={() => setType(t.id)}
                    className={`flex flex-col items-center gap-1 py-2.5 rounded-lg border transition-all ${
                      active
                        ? 'border-primary-500/50 bg-primary-500/10 text-primary-300'
                        : 'border-border bg-surface-subtle/50 text-text-muted hover:border-border-strong'
                    }`}
                  >
                    <Icon className="w-4 h-4" />
                    <span className="text-xs">{t.label}</span>
                  </button>
                )
              })}
            </div>
          </div>

          <div>
            <label className="text-xs text-text-muted mb-1.5 block">描述（可选）</label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="简单说说这个计划..."
              rows={3}
              className="w-full bg-surface-subtle border border-border rounded-lg px-3 py-2.5 text-sm
                         text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50
                         transition-colors resize-none"
            />
          </div>

          <button
            type="submit"
            disabled={!title.trim() || loading}
            className="w-full py-2.5 rounded-lg bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover
                       disabled:text-text-subtle text-white text-sm font-medium transition-colors
                       flex items-center justify-center gap-2"
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                创建中...
              </>
            ) : (
              <>
                <Plus className="w-4 h-4" />
                创建计划
              </>
            )}
          </button>
        </form>
      </div>
    </div>
  )
}

function MilestoneItem({ milestone, onComplete, onDelete }) {
  const completed = milestone.status === 'completed'

  return (
    <div className={`flex items-start gap-3 p-3 rounded-lg border transition-all ${
      completed ? 'bg-success-500/5 border-success-500/20' : 'bg-surface/40 border-border/60'
    }`}>
      <button
        onClick={() => !completed && onComplete(milestone.id)}
        disabled={completed}
        className={`mt-0.5 w-5 h-5 rounded-full border-2 flex items-center justify-center flex-shrink-0 transition-all ${
          completed
            ? 'bg-success-500 border-success-500 text-white'
            : 'border-border-strong hover:border-primary-400 hover:bg-primary-500/10'
        }`}
      >
        {completed && <Check className="w-3 h-3" />}
      </button>
      <div className="flex-1 min-w-0">
        <h4 className={`text-sm font-medium ${completed ? 'text-text-subtle line-through' : 'text-text'}`}>
          {milestone.title}
        </h4>
        {milestone.description && (
          <p className={`text-xs mt-1 ${completed ? 'text-text-subtle' : 'text-text-muted'}`}>
            {milestone.description}
          </p>
        )}
        {milestone.companion_comment && completed && (
          <p className="text-xs text-success-400/80 mt-2 italic">「{milestone.companion_comment}」</p>
        )}
      </div>
      {!completed && (
        <button
          onClick={() => onDelete(milestone.id)}
          className="text-text-subtle hover:text-danger-400 transition-colors p-1"
        >
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      )}
    </div>
  )
}

function CheckInItem({ checkIn, onDelete }) {
  return (
    <div className="relative pl-6 pb-5 border-l border-border last:pb-0">
      <div className="absolute -left-[5px] top-1 w-2.5 h-2.5 rounded-full bg-primary-500" />
      <div className="bg-surface/60 border border-border/60 rounded-lg p-3">
        <div className="flex items-center justify-between mb-2">
          <div className="flex items-center gap-2">
            <Calendar className="w-3.5 h-3.5 text-text-subtle" />
            <span className="text-xs text-text-muted">{formatDate(checkIn.date)}</span>
          </div>
          <button
            onClick={() => onDelete(checkIn.id)}
            className="text-text-subtle hover:text-danger-400 transition-colors p-1"
          >
            <Trash2 className="w-3 h-3" />
          </button>
        </div>
        <p className="text-sm text-text whitespace-pre-wrap leading-relaxed">{checkIn.content}</p>
        {checkIn.companion_response && (
          <div className="mt-3 pt-3 border-t border-border/50">
            <p className="text-xs text-primary-300/90 leading-relaxed">
              <span className="font-medium text-primary-400">Along：</span>
              {checkIn.companion_response}
            </p>
          </div>
        )}
      </div>
    </div>
  )
}

function PlanDetail({ goal, onBack, onUpdate, onDelete }) {
  const [milestones, setMilestones] = useState([])
  const [checkIns, setCheckIns] = useState([])
  const [activeTab, setActiveTab] = useState('overview')
  const [newMilestone, setNewMilestone] = useState('')
  const [newCheckIn, setNewCheckIn] = useState('')
  const [editing, setEditing] = useState(false)
  const [editTitle, setEditTitle] = useState(goal.title)
  const [editDesc, setEditDesc] = useState(goal.description || '')
  const [editStatus, setEditStatus] = useState(goal.status)
  const [editFocus, setEditFocus] = useState(goal.current_focus || '')
  const [editNextStep, setEditNextStep] = useState(goal.next_step || '')
  const [editProgress, setEditProgress] = useState(goal.progress || 0)

  const typeInfo = getTypeInfo(goal.type)
  const TypeIcon = typeInfo.icon

  const loadData = useCallback(async () => {
    if (!hasBackend()) return
    try {
      const [ms, ci] = await Promise.all([
        window.go.main.App.GetMilestones(goal.id),
        window.go.main.App.GetCheckIns(goal.id),
      ])
      setMilestones(Array.isArray(ms) ? ms : [])
      setCheckIns(Array.isArray(ci) ? ci : [])
    } catch (e) {
      console.error('加载失败:', e)
    }
  }, [goal.id])

  useEffect(() => {
    loadData()
  }, [loadData])

  const handleAddMilestone = async () => {
    if (!newMilestone.trim() || !hasBackend()) return
    try {
      await window.go.main.App.AddMilestone(goal.id, newMilestone.trim(), '')
      setNewMilestone('')
      loadData()
    } catch (e) {
      console.error('添加里程碑失败:', e)
    }
  }

  const handleCompleteMilestone = async (id) => {
    if (!hasBackend()) return
    try {
      await window.go.main.App.CompleteMilestone(id, '做得好！又前进了一步。')
      loadData()
      if (onUpdate) onUpdate()
    } catch (e) {
      console.error('完成里程碑失败:', e)
    }
  }

  const handleDeleteMilestone = async (id) => {
    if (!hasBackend()) return
    if (!confirm('确定删除这个里程碑吗？')) return
    try {
      await window.go.main.App.DeleteMilestone(id)
      loadData()
    } catch (e) {
      console.error('删除失败:', e)
    }
  }

  const handleAddCheckIn = async () => {
    if (!newCheckIn.trim() || !hasBackend()) return
    try {
      await window.go.main.App.AddCheckIn(goal.id, newCheckIn.trim(), '', '记录下来了，继续加油。')
      setNewCheckIn('')
      loadData()
    } catch (e) {
      console.error('添加记录失败:', e)
    }
  }

  const handleDeleteCheckIn = async (id) => {
    if (!hasBackend()) return
    if (!confirm('确定删除这条记录吗？')) return
    try {
      await window.go.main.App.DeleteCheckIn(id)
      loadData()
    } catch (e) {
      console.error('删除失败:', e)
    }
  }

  const handleSaveEdit = async () => {
    if (!hasBackend() || !editTitle.trim()) return
    try {
      await window.go.main.App.UpdateGoal(
        goal.id,
        editTitle.trim(),
        editDesc.trim(),
        editStatus,
        editFocus.trim(),
        editNextStep.trim(),
        '',
        parseInt(editProgress) || 0,
      )
      setEditing(false)
      if (onUpdate) onUpdate()
    } catch (e) {
      console.error('保存失败:', e)
    }
  }

  const handleDelete = async () => {
    if (!hasBackend()) return
    if (!confirm('确定删除这个计划吗？所有里程碑和记录都会被删除。')) return
    try {
      await window.go.main.App.DeleteGoal(goal.id)
      if (onDelete) onDelete(goal.id)
      onBack()
    } catch (e) {
      console.error('删除失败:', e)
    }
  }

  const completedCount = milestones.filter((m) => m.status === 'completed').length

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-2 mb-4">
        <button
          onClick={onBack}
          className="p-2 -ml-2 rounded-lg hover:bg-surface-subtle text-text-muted hover:text-text transition-colors"
        >
          <ChevronLeft className="w-5 h-5" />
        </button>
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2">
            <div className={`w-7 h-7 rounded-lg ${typeInfo.bg} flex items-center justify-center`}>
              <TypeIcon className={`w-4 h-4 ${typeInfo.color}`} />
            </div>
            <h2 className="font-semibold text-text truncate">{goal.title}</h2>
          </div>
        </div>
        <button
          onClick={() => setEditing(!editing)}
          className="p-2 rounded-lg hover:bg-surface-subtle text-text-muted hover:text-text transition-colors"
        >
          <Edit3 className="w-4 h-4" />
        </button>
      </div>

      {editing && (
        <div className="bg-surface/60 border border-border rounded-xl p-4 mb-4 space-y-3 animate-slide-up">
          <div>
            <label className="text-xs text-text-muted mb-1 block">名称</label>
            <input
              type="text"
              value={editTitle}
              onChange={(e) => setEditTitle(e.target.value)}
              className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm
                         text-text focus:outline-none focus:border-primary-500/50"
            />
          </div>
          <div>
            <label className="text-xs text-text-muted mb-1 block">描述</label>
            <textarea
              value={editDesc}
              onChange={(e) => setEditDesc(e.target.value)}
              rows={2}
              className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm
                         text-text focus:outline-none focus:border-primary-500/50 resize-none"
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-text-muted mb-1 block">状态</label>
              <select
                value={editStatus}
                onChange={(e) => setEditStatus(e.target.value)}
                className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm
                           text-text focus:outline-none focus:border-primary-500/50"
              >
                {Object.entries(STATUS_LABEL).map(([k, v]) => (
                  <option key={k} value={k}>{v}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="text-xs text-text-muted mb-1 block">进度 {editProgress}%</label>
              <input
                type="range"
                min="0"
                max="100"
                value={editProgress}
                onChange={(e) => setEditProgress(e.target.value)}
                className="w-full mt-2 accent-primary-500"
              />
            </div>
          </div>
          <div>
            <label className="text-xs text-text-muted mb-1 block">当前重点</label>
            <input
              type="text"
              value={editFocus}
              onChange={(e) => setEditFocus(e.target.value)}
              className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm
                         text-text focus:outline-none focus:border-primary-500/50"
            />
          </div>
          <div>
            <label className="text-xs text-text-muted mb-1 block">下一步</label>
            <input
              type="text"
              value={editNextStep}
              onChange={(e) => setEditNextStep(e.target.value)}
              className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm
                         text-text focus:outline-none focus:border-primary-500/50"
            />
          </div>
          <div className="flex gap-2 pt-1">
            <button
              onClick={handleSaveEdit}
              className="flex-1 py-2 rounded-lg bg-primary-600 hover:bg-primary-500 text-white text-sm font-medium transition-colors"
            >
              保存
            </button>
            <button
              onClick={handleDelete}
              className="px-4 py-2 rounded-lg bg-danger-500/10 hover:bg-danger-500/20 text-danger-400 text-sm transition-colors"
            >
              删除计划
            </button>
          </div>
        </div>
      )}

      <div className="bg-surface/40 border border-border/60 rounded-xl p-4 mb-4">
        <div className="flex items-center justify-between mb-3">
          <span className="text-xs text-text-muted">整体进度</span>
          <span className="text-sm font-medium text-primary-300">{goal.progress || 0}%</span>
        </div>
        <div className="h-2 bg-bg-hover rounded-full overflow-hidden mb-3">
          <div
            className="h-full bg-gradient-to-r from-primary-500 to-accent-400 rounded-full transition-all duration-700"
            style={{ width: `${goal.progress || 0}%` }}
          />
        </div>
        <div className="grid grid-cols-3 gap-3 text-center">
          <div>
            <div className="text-lg font-semibold text-text">{milestones.length}</div>
            <div className="text-xs text-text-subtle">里程碑</div>
          </div>
          <div>
            <div className="text-lg font-semibold text-success-400">{completedCount}</div>
            <div className="text-xs text-text-subtle">已完成</div>
          </div>
          <div>
            <div className="text-lg font-semibold text-accent-400">{checkIns.length}</div>
            <div className="text-xs text-text-subtle">条记录</div>
          </div>
        </div>
      </div>

      {goal.companion_note && !editing && (
        <div className="bg-primary-500/5 border border-primary-500/20 rounded-xl p-4 mb-4">
          <div className="flex items-start gap-3">
            <div className="w-8 h-8 rounded-full bg-primary-500/20 flex items-center justify-center flex-shrink-0">
              <MessageCircle className="w-4 h-4 text-primary-400" />
            </div>
            <div>
              <div className="text-xs text-primary-400 mb-1">Along 说</div>
              <p className="text-sm text-text leading-relaxed">{goal.companion_note}</p>
            </div>
          </div>
        </div>
      )}

      <div className="flex gap-1 mb-4 bg-surface/40 p-1 rounded-lg">
        {[
          { id: 'overview', label: '概览' },
          { id: 'milestones', label: '里程碑' },
          { id: 'journal', label: '记录' },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex-1 py-2 text-xs font-medium rounded-md transition-all ${
              activeTab === tab.id
                ? 'bg-bg-hover text-text'
                : 'text-text-muted hover:text-text'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'overview' && (
        <div className="space-y-4">
          {goal.description && (
            <div>
              <h4 className="text-xs font-medium text-text-muted mb-2">关于这个计划</h4>
              <p className="text-sm text-text leading-relaxed">{goal.description}</p>
            </div>
          )}

          {goal.current_focus && (
            <div>
              <h4 className="text-xs font-medium text-text-muted mb-2">当前重点</h4>
              <div className="flex items-center gap-2 p-3 bg-surface/60 rounded-lg border border-border/60">
                <Target className="w-4 h-4 text-accent-400" />
                <span className="text-sm text-text">{goal.current_focus}</span>
              </div>
            </div>
          )}

          {goal.next_step && (
            <div>
              <h4 className="text-xs font-medium text-text-muted mb-2">下一步</h4>
              <div className="flex items-center gap-2 p-3 bg-surface/60 rounded-lg border border-border/60">
                <ChevronRight className="w-4 h-4 text-primary-400" />
                <span className="text-sm text-text">{goal.next_step}</span>
              </div>
            </div>
          )}

          <div className="flex items-center gap-2 text-xs text-text-subtle pt-2">
            <Clock className="w-3.5 h-3.5" />
            <span>开始于 {formatDate(goal.start_date || goal.created_at)}</span>
          </div>
        </div>
      )}

      {activeTab === 'milestones' && (
        <div className="space-y-3">
          <div className="flex gap-2">
            <input
              type="text"
              value={newMilestone}
              onChange={(e) => setNewMilestone(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && handleAddMilestone()}
              placeholder="添加一个里程碑..."
              className="flex-1 bg-surface-subtle border border-border rounded-lg px-3 py-2 text-sm
                         text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50"
            />
            <button
              onClick={handleAddMilestone}
              disabled={!newMilestone.trim()}
              className="px-3 py-2 rounded-lg bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover
                         disabled:text-text-subtle text-white text-sm transition-colors"
            >
              <PlusCircle className="w-4 h-4" />
            </button>
          </div>

          {milestones.length === 0 ? (
            <div className="text-center py-8 text-text-subtle text-sm">
              还没有里程碑，添加第一个吧
            </div>
          ) : (
            <div className="space-y-2">
              {milestones.map((m) => (
                <MilestoneItem
                  key={m.id}
                  milestone={m}
                  onComplete={handleCompleteMilestone}
                  onDelete={handleDeleteMilestone}
                />
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === 'journal' && (
        <div className="space-y-3">
          <div>
            <textarea
              value={newCheckIn}
              onChange={(e) => setNewCheckIn(e.target.value)}
              placeholder="今天关于这个计划做了什么？或者有什么想法..."
              rows={3}
              className="w-full bg-surface-subtle border border-border rounded-xl px-3 py-2.5 text-sm
                         text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 resize-none"
            />
            <div className="flex justify-end mt-2">
              <button
                onClick={handleAddCheckIn}
                disabled={!newCheckIn.trim()}
                className="px-4 py-2 rounded-lg bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover
                           disabled:text-text-subtle text-white text-sm font-medium transition-colors"
              >
                记录下来
              </button>
            </div>
          </div>

          {checkIns.length === 0 ? (
            <div className="text-center py-8 text-text-subtle text-sm">
              还没有记录，写下第一条吧
            </div>
          ) : (
            <div className="pt-2">
              {checkIns.map((c) => (
                <CheckInItem key={c.id} checkIn={c} onDelete={handleDeleteCheckIn} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function PlanPage() {
  const [goals, setGoals] = useState([])
  const [filterType, setFilterType] = useState('all')
  const [selectedGoal, setSelectedGoal] = useState(null)
  const [showCreate, setShowCreate] = useState(false)
  const [loading, setLoading] = useState(true)

  const loadGoals = useCallback(async () => {
    if (!hasBackend()) {
      setGoals([])
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      const data = filterType === 'all'
        ? await window.go.main.App.GetGoals()
        : await window.go.main.App.GetGoalsByType(filterType)
      setGoals(Array.isArray(data) ? data : [])
    } catch (e) {
      console.error('加载计划失败:', e)
      setGoals([])
    } finally {
      setLoading(false)
    }
  }, [filterType])

  useEffect(() => {
    loadGoals()
  }, [loadGoals])

  const handleCreate = async (title, description, type) => {
    if (!hasBackend()) return
    try {
      await window.go.main.App.CreateGoal(title, description, type)
      loadGoals()
    } catch (e) {
      console.error('创建失败:', e)
    }
  }

  const handleDeleteGoal = (id) => {
    setGoals((prev) => prev.filter((g) => g.id !== id))
    setSelectedGoal(null)
  }

  const filteredGoals = goals

  return (
    <div className="max-w-2xl mx-auto py-4 px-4">
      {selectedGoal ? (
        <PlanDetail
          goal={selectedGoal}
          onBack={() => setSelectedGoal(null)}
          onUpdate={loadGoals}
          onDelete={handleDeleteGoal}
        />
      ) : (
        <>
          <div className="flex items-center justify-between mb-4">
            <div>
              <h1 className="text-xl font-bold text-text">我的计划</h1>
              <p className="text-xs text-text-subtle mt-0.5">和 Along 一起，一步步实现</p>
            </div>
            <button
              onClick={() => setShowCreate(true)}
              className="flex items-center gap-1.5 px-3 py-2 rounded-lg bg-primary-600 hover:bg-primary-500
                         text-white text-sm font-medium transition-colors shadow-lg shadow-primary-500/20"
            >
              <Plus className="w-4 h-4" />
              新建
            </button>
          </div>

          <div className="flex gap-1 mb-5 overflow-x-auto pb-1">
            <button
              onClick={() => setFilterType('all')}
              className={`px-3 py-1.5 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
                filterType === 'all'
                  ? 'bg-primary-500/15 text-primary-300 border border-primary-500/30'
                  : 'text-text-muted hover:text-text border border-transparent'
              }`}
            >
              全部
            </button>
            {PLAN_TYPES.map((t) => {
              const Icon = t.icon
              return (
                <button
                  key={t.id}
                  onClick={() => setFilterType(t.id)}
                  className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg whitespace-nowrap transition-all ${
                    filterType === t.id
                      ? `${t.bg} ${t.color} border border-${t.color}/30`
                      : 'text-text-muted hover:text-text border border-transparent'
                  }`}
                >
                  <Icon className="w-3.5 h-3.5" />
                  {t.label}
                </button>
              )
            })}
          </div>

          {loading ? (
            <div className="flex items-center justify-center py-16 text-text-subtle text-sm">
              <div className="w-4 h-4 border-2 border-border-strong border-t-primary-400 rounded-full animate-spin mr-2" />
              加载中...
            </div>
          ) : filteredGoals.length === 0 ? (
            <div className="text-center py-16">
              <div className="w-16 h-16 mx-auto mb-4 rounded-2xl bg-surface/60 flex items-center justify-center">
                <Target className="w-8 h-8 text-text-subtle" />
              </div>
              <h3 className="text-text-muted font-medium mb-1">还没有计划</h3>
              <p className="text-text-subtle text-sm mb-4">创建你的第一个计划吧</p>
              <button
                onClick={() => setShowCreate(true)}
                className="px-4 py-2 rounded-lg bg-primary-600 hover:bg-primary-500 text-white text-sm font-medium transition-colors"
              >
                创建计划
              </button>
            </div>
          ) : (
            <div className="grid gap-3">
              {filteredGoals.map((g) => (
                <PlanCard
                  key={g.id}
                  goal={g}
                  onClick={() => setSelectedGoal(g)}
                />
              ))}
            </div>
          )}
        </>
      )}

      {showCreate && (
        <CreatePlanModal
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </div>
  )
}

export default PlanPage
