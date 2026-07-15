import React, { useState, useEffect, useCallback } from 'react'
import {
  Plus, Play, Pause, Trash2, Edit3, Clock, Calendar, CheckCircle, AlertCircle,
  RefreshCw, ChevronRight, X, AlertTriangle, Info, ChevronDown, ChevronLeft,
  ArrowRight, Zap, MessageSquare, Search, FileText, Database, Bell, Activity,
  CheckSquare, BookOpen, GitBranch, Save, Loader2, Workflow, Settings,
} from 'lucide-react'

const hasBackend = () => {
  try {
    return typeof window !== 'undefined' && window.go && window.go.main && window.go.main.App
  } catch (e) {
    return false
  }
}

// 任务类型定义（带颜色区分，不使用粉色）
const TASK_TYPES = [
  {
    id: 'agent_chat', label: '对话', desc: '调用 Agent 进行对话交互',
    icon: MessageSquare,
    badge: 'text-blue-400 bg-blue-500/10 border-blue-500/30',
    iconBg: 'bg-blue-500/10 text-blue-400',
    accent: 'text-blue-400',
  },
  {
    id: 'web_search', label: '搜索', desc: '联网信息检索与汇总',
    icon: Search,
    badge: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/30',
    iconBg: 'bg-cyan-500/10 text-cyan-400',
    accent: 'text-cyan-400',
  },
  {
    id: 'report', label: '报告', desc: '生成日报/周报/月报',
    icon: FileText,
    badge: 'text-violet-400 bg-violet-500/10 border-violet-500/30',
    iconBg: 'bg-violet-500/10 text-violet-400',
    accent: 'text-violet-400',
  },
  {
    id: 'backup', label: '备份', desc: '数据备份归档',
    icon: Database,
    badge: 'text-orange-400 bg-orange-500/10 border-orange-500/30',
    iconBg: 'bg-orange-500/10 text-orange-400',
    accent: 'text-orange-400',
  },
  {
    id: 'reminder', label: '提醒', desc: '定时消息提醒',
    icon: Bell,
    badge: 'text-yellow-400 bg-yellow-500/10 border-yellow-500/30',
    iconBg: 'bg-yellow-500/10 text-yellow-400',
    accent: 'text-yellow-400',
  },
  {
    id: 'monitor', label: '监控', desc: '网页/RSS/文件变更监控',
    icon: Activity,
    badge: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/30',
    iconBg: 'bg-emerald-500/10 text-emerald-400',
    accent: 'text-emerald-400',
  },
  {
    id: 'habit_checkin', label: '打卡', desc: '习惯打卡与记录',
    icon: CheckSquare,
    badge: 'text-red-400 bg-red-500/10 border-red-500/30',
    iconBg: 'bg-red-500/10 text-red-400',
    accent: 'text-red-400',
  },
  {
    id: 'review', label: '复习', desc: '艾宾浩斯复习计划',
    icon: BookOpen,
    badge: 'text-indigo-400 bg-indigo-500/10 border-indigo-500/30',
    iconBg: 'bg-indigo-500/10 text-indigo-400',
    accent: 'text-indigo-400',
  },
  {
    id: 'cleanup', label: '清理', desc: '清理过期数据与临时文件',
    icon: Trash2,
    badge: 'text-gray-400 bg-gray-500/10 border-gray-500/30',
    iconBg: 'bg-gray-500/10 text-gray-400',
    accent: 'text-gray-400',
  },
  {
    id: 'workflow', label: '流程', desc: '多步骤任务编排',
    icon: GitBranch,
    badge: 'text-blue-300 bg-blue-700/10 border-blue-700/40',
    iconBg: 'bg-blue-700/10 text-blue-300',
    accent: 'text-blue-300',
  },
]

const getTypeInfo = (type) => TASK_TYPES.find((t) => t.id === type) || TASK_TYPES[0]

// 流程步骤类型
const STEP_TYPES = [
  { id: 'agent', label: 'Agent 调用', icon: MessageSquare, desc: '调用指定 Agent 执行' },
  { id: 'search', label: '网络搜索', icon: Search, desc: '搜索互联网信息' },
  { id: 'condition', label: '条件判断', icon: GitBranch, desc: '根据条件分支跳转' },
  { id: 'save_file', label: '保存文件', icon: Save, desc: '保存内容到文件' },
  { id: 'notify', label: '发送通知', icon: Bell, desc: '推送消息提醒' },
]

const getStepTypeInfo = (type) => STEP_TYPES.find((t) => t.id === type) || STEP_TYPES[0]

const WEEKDAYS = [
  { value: 1, label: '周一' },
  { value: 2, label: '周二' },
  { value: 3, label: '周三' },
  { value: 4, label: '周四' },
  { value: 5, label: '周五' },
  { value: 6, label: '周六' },
  { value: 0, label: '周日' },
]

const SCHEDULE_TYPES = [
  { value: 'once', label: '仅一次' },
  { value: 'daily', label: '每天' },
  { value: 'weekly', label: '每周' },
  { value: 'monthly', label: '每月' },
  { value: 'custom', label: '自定义' },
]

const AGENTS = [
  { value: 'web', label: 'Web Agent' },
  { value: 'planner', label: '计划 Agent' },
  { value: 'emotion', label: '情感 Agent' },
  { value: 'memory', label: '记忆 Agent' },
  { value: 'reflection', label: '反思 Agent' },
  { value: 'summarize', label: '总结 Agent' },
  { value: 'tool', label: '工具 Agent' },
]

// ---------- 工具函数 ----------
function formatDateTime(t) {
  if (!t || t === '0001-01-01T00:00:00Z' || t === '') return '-'
  try {
    const date = new Date(t)
    if (isNaN(date.getTime())) return '-'
    return date.toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  } catch (e) {
    return '-'
  }
}

function formatScheduleText(task) {
  if (!task) return '-'
  const type = task.schedule_type
  let cfg = {}
  try {
    cfg = task.schedule_config ? JSON.parse(task.schedule_config) : {}
  } catch (e) {
    cfg = {}
  }
  switch (type) {
    case 'once':
      return `仅一次 ${formatDateTime(cfg.datetime)}`
    case 'daily':
      return `每天 ${cfg.time || '09:00'}`
    case 'weekly': {
      if (Array.isArray(cfg.days) && cfg.days.length > 0) {
        const dayLabels = cfg.days.map(d => {
          const day = WEEKDAYS.find((w) => w.value === d)
          return day ? day.label : `周${d}`
        }).sort()
        return `${dayLabels.join('、')} ${cfg.time || '09:00'}`
      }
      const day = WEEKDAYS.find((w) => w.value === (cfg.day ?? 1))
      return `每${day ? day.label : '周一'} ${cfg.time || '09:00'}`
    }
    case 'monthly':
      return `每月${cfg.day || 1}日 ${cfg.time || '09:00'}`
    case 'custom':
      return cfg.cron || '自定义'
    default:
      return type || '-'
  }
}

// 解析条件 "key=value" 判断当前 config 是否满足
function isConditionMet(condition, config) {
  if (!condition) return true
  const idx = condition.indexOf('=')
  if (idx === -1) return true
  const key = condition.substring(0, idx).trim()
  const value = condition.substring(idx + 1).trim()
  const actual = config[key]
  // 处理 boolean / string
  if (actual === true) return String(true) === value
  if (actual === false) return String(false) === value
  return String(actual ?? '') === value
}

function formatDuration(ms) {
  if (!ms || ms <= 0) return '-'
  if (ms < 1000) return `${ms}ms`
  const s = ms / 1000
  if (s < 60) return `${s.toFixed(1)}s`
  const m = Math.floor(s / 60)
  const rest = Math.round(s % 60)
  return `${m}m${rest}s`
}

// ---------- 动态配置表单 ----------
function ConfigForm({ fields, config, setConfig }) {
  if (!fields || fields.length === 0) {
    return (
      <div className="text-sm text-text-subtle bg-surface-subtle rounded-lg p-3 flex items-center gap-2">
        <Info className="w-4 h-4" />
        <span>该任务类型无需额外配置</span>
      </div>
    )
  }

  const updateField = (key, value) => setConfig({ ...config, [key]: value })

  return (
    <div className="space-y-4">
      {fields.map((field) => {
        if (!isConditionMet(field.condition, config)) return null
        const required = field.required
        const labelTxt = field.label + (required ? ' *' : '')
        const value = config[field.key]

        return (
          <div key={field.key}>
            <label className="block text-sm font-medium text-text mb-1.5">{labelTxt}</label>
            {renderField(field, value, updateField)}
            {field.placeholder && field.type !== 'select' && field.type !== 'boolean' && (
              <p className="text-xs text-text-subtle mt-1">{field.placeholder}</p>
            )}
          </div>
        )
      })}
    </div>
  )
}

function renderField(field, value, updateField) {
  const baseInput =
    'w-full px-3 py-2.5 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all'

  switch (field.type) {
    case 'text':
      return (
        <input
          type="text"
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          placeholder={field.placeholder || ''}
          className={baseInput}
        />
      )
    case 'textarea':
      return (
        <textarea
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          placeholder={field.placeholder || ''}
          rows={3}
          className={baseInput + ' resize-none font-mono'}
        />
      )
    case 'number':
      return (
        <input
          type="number"
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value === '' ? '' : Number(e.target.value))}
          placeholder={field.placeholder || ''}
          className={baseInput}
        />
      )
    case 'select':
      return (
        <select
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          className={baseInput}
        >
          <option value="">请选择...</option>
          {(field.options || []).map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      )
    case 'multi_select': {
      const arr = Array.isArray(value) ? value : []
      const toggle = (val) => {
        if (arr.includes(val)) updateField(field.key, arr.filter((v) => v !== val))
        else updateField(field.key, [...arr, val])
      }
      return (
        <div className="flex flex-wrap gap-2">
          {(field.options || []).map((opt) => {
            const active = arr.includes(opt.value)
            return (
              <button
                key={opt.value}
                type="button"
                onClick={() => toggle(opt.value)}
                className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-all ${
                  active
                    ? 'bg-primary-500/15 border-primary-500 text-primary-400'
                    : 'bg-bg border-border text-text-muted hover:border-border-strong hover:text-text'
                }`}
              >
                {opt.label}
              </button>
            )
          })}
        </div>
      )
    }
    case 'boolean':
      return (
        <button
          type="button"
          onClick={() => updateField(field.key, !value)}
          className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
            value ? 'bg-primary-500' : 'bg-border-strong'
          }`}
        >
          <span
            className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
              value ? 'translate-x-6' : 'translate-x-1'
            }`}
          />
        </button>
      )
    case 'date':
      return (
        <input
          type="date"
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          className={baseInput}
        />
      )
    case 'time':
      return (
        <input
          type="time"
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          className={baseInput}
        />
      )
    default:
      return (
        <input
          type="text"
          value={value ?? ''}
          onChange={(e) => updateField(field.key, e.target.value)}
          className={baseInput}
        />
      )
  }
}

// ---------- 调度编辑器 ----------
function ScheduleEditor({ scheduleType, scheduleConfig, setScheduleType, setScheduleConfig }) {
  const update = (patch) => setScheduleConfig({ ...scheduleConfig, ...patch })
  const labelCls = 'block text-sm font-medium text-text mb-1.5'
  const inputCls =
    'w-full px-3 py-2.5 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all'

  return (
    <div className="space-y-4">
      <div>
        <label className={labelCls}>重复方式</label>
        <div className="flex flex-wrap gap-2">
          {SCHEDULE_TYPES.map((s) => (
            <button
              key={s.value}
              type="button"
              onClick={() => setScheduleType(s.value)}
              className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-all ${
                scheduleType === s.value
                  ? 'bg-primary-500/15 border-primary-500 text-primary-400'
                  : 'bg-bg border-border text-text-muted hover:border-border-strong hover:text-text'
              }`}
            >
              {s.label}
            </button>
          ))}
        </div>
      </div>

      {scheduleType === 'once' && (
        <div>
          <label className={labelCls}>执行时间</label>
          <input
            type="datetime-local"
            value={scheduleConfig.datetime || ''}
            onChange={(e) => update({ datetime: e.target.value })}
            className={inputCls}
          />
        </div>
      )}

      {scheduleType === 'daily' && (
        <div>
          <label className={labelCls}>每日执行时间</label>
          <input
            type="time"
            value={scheduleConfig.time || '09:00'}
            onChange={(e) => update({ time: e.target.value })}
            className={inputCls}
          />
        </div>
      )}

      {scheduleType === 'weekly' && (
        <div className="space-y-3">
          <div>
            <label className={labelCls}>星期（可多选）</label>
            <div className="flex flex-wrap gap-2">
              {WEEKDAYS.map((w) => {
                const daysArr = Array.isArray(scheduleConfig.days) ? scheduleConfig.days : []
                const active = daysArr.includes(w.value) || scheduleConfig.day === w.value
                const handleClick = () => {
                  const current = Array.isArray(scheduleConfig.days) ? [...scheduleConfig.days] : (scheduleConfig.day !== undefined ? [scheduleConfig.day] : [])
                  if (active) {
                    update({ days: current.filter(d => d !== w.value), day: undefined })
                  } else {
                    update({ days: [...current, w.value], day: undefined })
                  }
                }
                return (
                  <button
                    key={w.value}
                    type="button"
                    onClick={handleClick}
                    className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-all ${
                      active
                        ? 'bg-primary-500/15 border-primary-500 text-primary-400'
                        : 'bg-bg border-border text-text-muted hover:border-border-strong hover:text-text'
                    }`}
                  >
                    {w.label}
                  </button>
                )
              })}
            </div>
          </div>
          <div>
            <label className={labelCls}>执行时间</label>
            <input
              type="time"
              value={scheduleConfig.time || '09:00'}
              onChange={(e) => update({ time: e.target.value })}
              className={inputCls}
            />
          </div>
        </div>
      )}

      {scheduleType === 'monthly' && (
        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className={labelCls}>日期（每月几日）</label>
            <input
              type="number"
              min="1"
              max="31"
              value={scheduleConfig.day ?? 1}
              onChange={(e) => update({ day: Number(e.target.value) })}
              className={inputCls}
            />
          </div>
          <div>
            <label className={labelCls}>执行时间</label>
            <input
              type="time"
              value={scheduleConfig.time || '09:00'}
              onChange={(e) => update({ time: e.target.value })}
              className={inputCls}
            />
          </div>
        </div>
      )}

      {scheduleType === 'custom' && (
        <div>
          <label className={labelCls}>
            Cron 表达式
            <span className="text-text-muted font-normal ml-1">（分 时 日 月 周）</span>
          </label>
          <input
            type="text"
            value={scheduleConfig.cron || ''}
            onChange={(e) => update({ cron: e.target.value })}
            placeholder="例如：0 8 * * *"
            className={inputCls + ' font-mono'}
          />
          <div className="mt-2 flex flex-wrap gap-1.5">
            {[
              { label: '每天 8:00', value: '0 8 * * *' },
              { label: '每天 20:00', value: '0 20 * * *' },
              { label: '每小时', value: '0 * * * *' },
              { label: '每30分钟', value: '*/30 * * * *' },
            ].map((p) => (
              <button
                key={p.value}
                type="button"
                onClick={() => update({ cron: p.value })}
                className={`px-2.5 py-1 text-xs rounded-lg transition-colors ${
                  scheduleConfig.cron === p.value
                    ? 'bg-primary-500 text-white'
                    : 'bg-surface-subtle text-text-muted hover:text-text hover:bg-surface-hover'
                }`}
              >
                {p.label}
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  )
}

// ---------- 任务卡片 ----------
function TaskCard({
  task,
  expanded,
  executions,
  loadingExecutions,
  onToggleExpand,
  onEdit,
  onDelete,
  onToggle,
  onRunNow,
  onEditWorkflow,
}) {
  const typeInfo = getTypeInfo(task.task_type)
  const TypeIcon = typeInfo.icon
  const isEnabled = task.enabled

  return (
    <div
      className={`bg-surface border rounded-xl transition-all duration-200 ${
        isEnabled
          ? 'border-border hover:border-primary-400/50 hover:shadow-lg hover:shadow-primary-500/5'
          : 'border-border-subtle opacity-75'
      }`}
    >
      <div className="p-4">
        <div className="flex items-start justify-between">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-2">
              <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${typeInfo.iconBg}`}>
                <TypeIcon className="w-4 h-4" />
              </div>
              <div className="flex-1 min-w-0">
                <h3 className={`font-medium truncate ${isEnabled ? 'text-text' : 'text-text-muted'}`}>
                  {task.name}
                </h3>
                <span
                  className={`inline-block mt-0.5 px-2 py-0.5 rounded-full text-xs font-medium border ${typeInfo.badge}`}
                >
                  {typeInfo.label}
                </span>
              </div>
              <span
                className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                  isEnabled
                    ? 'bg-success-500/10 text-success-400'
                    : 'bg-warning-500/10 text-warning-400'
                }`}
              >
                {isEnabled ? '运行中' : '已暂停'}
              </span>
              {task.last_run_at && task.last_run_at !== '0001-01-01T00:00:00Z' && (
                <span className={`inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs font-medium ${
                  task.status === 'success'
                    ? 'bg-green-500/10 text-green-400'
                    : task.status === 'failed'
                    ? 'bg-red-500/10 text-red-400'
                    : 'bg-gray-500/10 text-gray-400'
                }`}>
                  {task.status === 'success' ? (
                    <CheckCircle className="w-3 h-3" />
                  ) : task.status === 'failed' ? (
                    <AlertCircle className="w-3 h-3" />
                  ) : null}
                  {task.status === 'success' ? '成功' : task.status === 'failed' ? '失败' : '未知'}
                </span>
              )}
            </div>

            {task.description && (
              <p className="text-sm text-text-subtle mb-3 line-clamp-2 pl-10">{task.description}</p>
            )}

            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-text-muted pl-10">
              <div className="flex items-center gap-1.5">
                <Calendar className="w-3.5 h-3.5" />
                <span>{formatScheduleText(task)}</span>
              </div>
              {task.last_run_at && task.last_run_at !== '0001-01-01T00:00:00Z' && (
                <div className="flex items-center gap-1.5">
                  <Clock className="w-3.5 h-3.5" />
                  <span>上次: {formatDateTime(task.last_run_at)}</span>
                </div>
              )}
              {task.next_run_at && task.next_run_at !== '0001-01-01T00:00:00Z' && isEnabled && (
                <div className="flex items-center gap-1.5">
                  <ArrowRight className="w-3.5 h-3.5" />
                  <span>下次: {formatDateTime(task.next_run_at)}</span>
                </div>
              )}
            </div>
          </div>

          <div className="flex items-center gap-1 ml-4 flex-shrink-0">
            <button
              onClick={() => onRunNow(task.id)}
              className="p-2 text-text-muted hover:text-primary-400 hover:bg-primary-500/10 rounded-lg transition-colors"
              title="立即执行"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
            {task.task_type === 'workflow' && (
              <button
                onClick={() => onEditWorkflow(task)}
                className="p-2 text-text-muted hover:text-violet-400 hover:bg-violet-500/10 rounded-lg transition-colors"
                title="编辑流程"
              >
                <Workflow className="w-4 h-4" />
              </button>
            )}
            <button
              onClick={() => onToggleExpand(task.id)}
              className="p-2 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
              title="执行记录"
            >
              {expanded ? <ChevronDown className="w-4 h-4" /> : <ChevronRight className="w-4 h-4" />}
            </button>
            <button
              onClick={() => onEdit(task)}
              className="p-2 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
              title="编辑"
            >
              <Edit3 className="w-4 h-4" />
            </button>
            <button
              onClick={() => onDelete(task.id)}
              className="p-2 text-text-muted hover:text-danger-400 hover:bg-danger-500/10 rounded-lg transition-colors"
              title="删除"
            >
              <Trash2 className="w-4 h-4" />
            </button>
            <button
              onClick={() => onToggle(task.id, !isEnabled)}
              className={`p-2 rounded-lg transition-colors ${
                isEnabled
                  ? 'text-success-400 hover:bg-success-500/10'
                  : 'text-warning-400 hover:bg-warning-500/10'
              }`}
              title={isEnabled ? '暂停' : '启用'}
            >
              {isEnabled ? <Pause className="w-4 h-4" /> : <Play className="w-4 h-4" />}
            </button>
          </div>
        </div>
      </div>

      {expanded && (
        <div className="border-t border-border bg-bg-subtle/50 p-4 animate-slide-down">
          <div className="text-xs font-medium text-text-muted mb-2 flex items-center gap-1.5">
            <Clock className="w-3.5 h-3.5" />
            执行记录
          </div>
          {loadingExecutions ? (
            <div className="flex items-center justify-center py-6 text-text-subtle text-sm">
              <Loader2 className="w-4 h-4 animate-spin mr-2" />
              加载中...
            </div>
          ) : executions.length === 0 ? (
            <div className="text-center py-6 text-text-subtle text-sm">
              暂无执行记录
            </div>
          ) : (
            <div className="space-y-2 max-h-72 overflow-y-auto">
              {executions.map((exec) => (
                <ExecutionItem key={exec.id} exec={exec} isWorkflow={task.task_type === 'workflow'} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ---------- 执行记录项 ----------
function ExecutionItem({ exec, isWorkflow }) {
  const [showSteps, setShowSteps] = useState(false)
  const [steps, setSteps] = useState([])
  const [loadingSteps, setLoadingSteps] = useState(false)

  const status = exec.status
  const isSuccess = status === 'success'
  const isFailed = status === 'failed' || status === 'error'

  const loadSteps = async () => {
    if (!hasBackend()) return
    setLoadingSteps(true)
    try {
      const result = await window.go.main.App.GetStepExecutions(exec.id)
      setSteps(Array.isArray(result) ? result : [])
    } catch (e) {
      setSteps([])
    } finally {
      setLoadingSteps(false)
    }
  }

  const toggleSteps = () => {
    if (!showSteps && isWorkflow && steps.length === 0) {
      loadSteps()
    }
    setShowSteps(!showSteps)
  }

  return (
    <div
      className={`bg-surface border rounded-lg p-3 ${
        isSuccess
          ? 'border-success-500/30'
          : isFailed
          ? 'border-danger-500/30'
          : 'border-border'
      }`}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          {isSuccess ? (
            <CheckCircle className="w-4 h-4 text-success-500 flex-shrink-0" />
          ) : isFailed ? (
            <AlertCircle className="w-4 h-4 text-danger-500 flex-shrink-0" />
          ) : (
            <Loader2 className="w-4 h-4 text-primary-400 animate-spin flex-shrink-0" />
          )}
          <span
            className={`text-xs font-medium ${
              isSuccess ? 'text-success-500' : isFailed ? 'text-danger-500' : 'text-primary-400'
            }`}
          >
            {isSuccess ? '成功' : isFailed ? '失败' : '运行中'}
          </span>
          <span className="text-xs text-text-muted">{formatDateTime(exec.started_at)}</span>
          <span className="text-xs text-text-subtle">·</span>
          <span className="text-xs text-text-muted">耗时 {formatDuration(exec.duration_ms)}</span>
        </div>
        {isWorkflow && (
          <button
            onClick={toggleSteps}
            className="text-xs text-primary-400 hover:text-primary-300 flex items-center gap-1"
          >
            {showSteps ? '收起' : '查看详情'}
            <ChevronDown className={`w-3 h-3 transition-transform ${showSteps ? 'rotate-180' : ''}`} />
          </button>
        )}
      </div>

      {exec.result_content && (
        <p className="text-sm text-text line-clamp-3 bg-surface-subtle p-2 rounded-lg mb-1">
          {exec.result_content}
        </p>
      )}
      {exec.result_path && (
        <div className="flex items-center gap-1.5 text-xs text-primary-400 mb-1">
          <FileText className="w-3.5 h-3.5" />
          <span className="truncate font-mono">{exec.result_path}</span>
        </div>
      )}
      {exec.error_message && (
        <div className="flex items-start gap-2 bg-danger-500/10 p-2 rounded-lg">
          <AlertTriangle className="w-4 h-4 text-danger-500 flex-shrink-0 mt-0.5" />
          <p className="text-xs text-danger-500">{exec.error_message}</p>
        </div>
      )}

      {showSteps && isWorkflow && (
        <div className="mt-2 border-t border-border pt-2">
          {loadingSteps ? (
            <div className="text-xs text-text-subtle text-center py-2">加载步骤...</div>
          ) : steps.length === 0 ? (
            <div className="text-xs text-text-subtle text-center py-2">无步骤记录</div>
          ) : (
            <div className="space-y-1.5">
              {steps.map((s, idx) => {
                const sSuccess = s.status === 'success'
                return (
                  <div
                    key={s.id || idx}
                    className="flex items-center gap-2 text-xs bg-bg p-2 rounded-lg border border-border"
                  >
                    <span className="w-5 h-5 flex items-center justify-center bg-primary-500/10 text-primary-400 rounded-full text-xs font-medium">
                      {idx + 1}
                    </span>
                    {sSuccess ? (
                      <CheckCircle className="w-3.5 h-3.5 text-success-500" />
                    ) : (
                      <AlertCircle className="w-3.5 h-3.5 text-danger-500" />
                    )}
                    <span className="text-text">{s.step_name || `步骤${idx + 1}`}</span>
                    <span className="text-text-muted">·</span>
                    <span className="text-text-muted">{formatDuration(s.duration_ms)}</span>
                    {s.error_message && (
                      <span className="text-danger-400 truncate">- {s.error_message}</span>
                    )}
                  </div>
                )
              })}
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ---------- 新建/编辑任务弹窗（多步骤） ----------
function TaskWizard({ isOpen, onClose, onSubmit, editingTask }) {
  const [step, setStep] = useState(0)
  const [selectedTemplate, setSelectedTemplate] = useState(null)
  const [templates, setTemplates] = useState([])
  const [name, setName] = useState('')
  const [description, setDescription] = useState('')
  const [taskType, setTaskType] = useState('workflow')
  const [config, setConfig] = useState({})
  const [scheduleType, setScheduleType] = useState('daily')
  const [scheduleConfig, setScheduleConfig] = useState({ time: '09:00' })
  const [enabled, setEnabled] = useState(true)
  const [slashCommand, setSlashCommand] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [steps, setSteps] = useState([])
  const [loadingTemplates, setLoadingTemplates] = useState(false)

  useEffect(() => {
    if (!isOpen) return
    if (editingTask) {
      setName(editingTask.name || '')
      setDescription(editingTask.description || '')
      setTaskType(editingTask.task_type || 'workflow')
      try {
        setConfig(editingTask.config ? JSON.parse(editingTask.config) : {})
      } catch (e) {
        setConfig({})
      }
      setScheduleType(editingTask.schedule_type || 'daily')
      try {
        setScheduleConfig(editingTask.schedule_config ? JSON.parse(editingTask.schedule_config) : { time: '09:00' })
      } catch (e) {
        setScheduleConfig({ time: '09:00' })
      }
      setEnabled(editingTask.enabled !== false)
      setSlashCommand(editingTask.slash_command || '')
      try {
        setSteps(editingTask.steps ? JSON.parse(editingTask.steps) : [])
      } catch (e) {
        setSteps([])
      }
      setSelectedTemplate(null)
      setStep(1)
    } else {
      setName('')
      setDescription('')
      setTaskType('workflow')
      setConfig({})
      setSteps([])
      setScheduleType('daily')
      setScheduleConfig({ time: '09:00' })
      setEnabled(true)
      setSlashCommand('')
      setSelectedTemplate(null)
      setStep(0)
      loadTemplates()
    }
  }, [isOpen, editingTask])

  const loadTemplates = async () => {
    if (!hasBackend()) return
    setLoadingTemplates(true)
    try {
      const result = await window.go.main.App.GetTaskTemplates()
      setTemplates(Array.isArray(result) ? result : [])
    } catch (e) {
      console.error('加载模板失败:', e)
      setTemplates([])
    } finally {
      setLoadingTemplates(false)
    }
  }

  const handleSelectTemplate = (template) => {
    setSelectedTemplate(template)
    setName(template.name || '')
    setDescription(template.description || '')
    setTaskType(template.task_type || 'workflow')
    setScheduleType(template.default_schedule_type || 'daily')
    try {
      const cfg = template.default_config ? JSON.parse(template.default_config) : {}
      setConfig(cfg)
    } catch (e) {
      setConfig({})
    }
    try {
      const scfg = template.default_schedule_config ? JSON.parse(template.default_schedule_config) : { time: '09:00' }
      setScheduleConfig(scfg)
    } catch (e) {
      setScheduleConfig({ time: '09:00' })
    }
    try {
      const s = template.steps ? JSON.parse(template.steps) : []
      setSteps(s)
    } catch (e) {
      setSteps([])
    }
  }

  if (!isOpen) return null

  const handleSubmit = async () => {
    if (!name.trim()) {
      alert('请填写任务名称')
      return
    }
    setSubmitting(true)
    try {
      const configStr = JSON.stringify(config)
      const scheduleConfigStr = JSON.stringify(scheduleConfig)
      await onSubmit({
        name: name.trim(),
        description: description.trim(),
        taskType,
        config: configStr,
        scheduleType,
        scheduleConfig: scheduleConfigStr,
        enabled,
        slashCommand: slashCommand.trim(),
        templateID: selectedTemplate ? selectedTemplate.id : null,
      })
      onClose()
    } catch (e) {
      alert('保存失败: ' + (e.message || e))
    } finally {
      setSubmitting(false)
    }
  }

  const canGoNext = () => {
    if (step === 0) return !!selectedTemplate
    if (step === 1) return !!name.trim()
    if (step === 2) return true
    return true
  }

  const isCustomWorkflow = selectedTemplate?.name === '自定义工作流' || taskType === 'workflow'

  return (
    <div className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-surface border border-border rounded-xl w-full max-w-2xl max-h-[90vh] flex flex-col overflow-hidden animate-fade-in">
        {/* 头部 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-bg">
          <div className="flex items-center gap-3">
            <h3 className="font-semibold text-text">
              {editingTask ? '编辑自动化任务' : '新建自动化任务'}
            </h3>
            <div className="flex items-center gap-1.5 text-xs text-text-muted">
              <span className={step >= 0 ? 'text-primary-400' : ''}>模板</span>
              <ChevronRight className="w-3 h-3" />
              <span className={step >= 1 ? 'text-primary-400' : ''}>配置</span>
              <ChevronRight className="w-3 h-3" />
              <span className={step >= 2 ? 'text-primary-400' : ''}>调度</span>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* 内容 */}
        <div className="flex-1 overflow-y-auto p-4">
          {step === 0 && (
            <div>
              <label className="block text-sm font-medium text-text mb-3">选择模板</label>
              {loadingTemplates ? (
                <div className="flex items-center justify-center py-12 text-text-subtle text-sm">
                  <Loader2 className="w-4 h-4 animate-spin mr-2" />
                  加载模板中...
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                  {templates.map((t) => (
                    <button
                      key={t.id}
                      onClick={() => handleSelectTemplate(t)}
                      className={`p-4 border rounded-xl text-left transition-all ${
                        selectedTemplate?.id === t.id
                          ? 'border-primary-500 bg-primary-500/10 ring-2 ring-primary-500/20'
                          : 'border-border hover:border-border-strong hover:bg-surface-subtle'
                      }`}
                    >
                      <div className="text-3xl mb-3">{t.icon}</div>
                      <div className="font-medium text-text mb-1">{t.name}</div>
                      <div className="text-xs text-text-muted line-clamp-2">{t.description}</div>
                    </button>
                  ))}
                </div>
              )}
              {templates.length === 0 && !loadingTemplates && (
                <div className="text-center py-8 text-text-subtle text-sm">
                  暂无模板，请联系管理员添加
                </div>
              )}
            </div>
          )}

          {step === 1 && (
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-text mb-1.5">任务名称</label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="例如：每日新闻简报"
                  className="w-full px-3 py-2.5 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all"
                  required
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-text mb-1.5">描述</label>
                <textarea
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  placeholder="描述这个任务的用途..."
                  rows={2}
                  className="w-full px-3 py-2.5 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all resize-none"
                />
              </div>

              {!isCustomWorkflow && steps.length > 0 && (
                <div>
                  <label className="block text-sm font-medium text-text mb-3">任务步骤预览</label>
                  <div className="space-y-2 bg-surface-subtle rounded-lg p-3">
                    {steps.map((s, idx) => (
                      <div key={idx} className="flex items-center gap-3 text-sm">
                        <span className="w-6 h-6 flex items-center justify-center bg-primary-500/10 text-primary-400 text-xs font-medium rounded-full flex-shrink-0">
                          {idx + 1}
                        </span>
                        <span className="text-text">{s.name || `步骤${idx + 1}`}</span>
                        <span className="text-xs text-text-muted">· {s.step_type || 'agent'}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}

              {isCustomWorkflow && (
                <div className="text-sm text-text-subtle bg-violet-500/10 border border-violet-500/20 rounded-lg p-3 flex items-start gap-2">
                  <Info className="w-4 h-4 text-violet-400 flex-shrink-0 mt-0.5" />
                  <span>
                    自定义工作流的步骤需在创建后通过「编辑流程」按钮配置。这里只需填写名称和描述。
                  </span>
                </div>
              )}

              <div>
                <label className="block text-sm font-medium text-text mb-1.5">斜杠命令</label>
                <div className="flex items-center gap-2">
                  <span className="text-sm bg-bg px-3 py-2.5 rounded-l-lg border border-r-0 border-border text-text-muted">/</span>
                  <input
                    type="text"
                    value={slashCommand}
                    onChange={(e) => setSlashCommand(e.target.value)}
                    placeholder="research"
                    className="flex-1 px-3 py-2.5 bg-bg border border-border rounded-r-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all"
                  />
                </div>
                <p className="text-xs text-text-subtle mt-1">对话中输入 /命令名 可直接触发此任务</p>
              </div>
            </div>
          )}

          {step === 2 && (
            <div className="space-y-4">
              <ScheduleEditor
                scheduleType={scheduleType}
                scheduleConfig={scheduleConfig}
                setScheduleType={setScheduleType}
                setScheduleConfig={setScheduleConfig}
              />

              <div className="flex items-center gap-2 p-3 bg-surface-subtle rounded-lg">
                <button
                  type="button"
                  onClick={() => setEnabled(!enabled)}
                  className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                    enabled ? 'bg-primary-500' : 'bg-border-strong'
                  }`}
                >
                  <span
                    className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                      enabled ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
                <label className="text-sm text-text">
                  {editingTask ? '启用任务' : '创建后立即启用'}
                </label>
              </div>
            </div>
          )}
        </div>

        {/* 底部按钮 */}
        <div className="flex items-center justify-between px-4 py-3 border-t border-border">
          <div>
            {step > 0 && (!editingTask || step > 1) && (
              <button
                type="button"
                onClick={() => setStep(step - 1)}
                className="flex items-center gap-1 px-3 py-2 text-sm text-text-muted hover:text-text transition-colors"
              >
                <ChevronLeft className="w-4 h-4" />
                上一步
              </button>
            )}
          </div>
          <div className="flex items-center gap-2">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
            >
              取消
            </button>
            {step < 2 ? (
              <button
                type="button"
                onClick={() => setStep(step + 1)}
                disabled={!canGoNext()}
                className="px-4 py-2 bg-primary-600 hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
              >
                下一步
              </button>
            ) : (
              <button
                type="button"
                onClick={handleSubmit}
                disabled={submitting}
                className="px-4 py-2 bg-primary-600 hover:bg-primary-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors flex items-center gap-2"
              >
                {submitting && <Loader2 className="w-4 h-4 animate-spin" />}
                {editingTask ? '保存修改' : '创建任务'}
              </button>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

// ---------- 流程编辑器 ----------
function WorkflowEditor({ isOpen, onClose, task }) {
  const [steps, setSteps] = useState([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [selectedIdx, setSelectedIdx] = useState(-1)
  const [dragIdx, setDragIdx] = useState(-1)

  const loadSteps = useCallback(async () => {
    if (!task || !hasBackend()) {
      setLoading(false)
      return
    }
    setLoading(true)
    try {
      const result = await window.go.main.App.GetAutomationSteps(task.id)
      const arr = Array.isArray(result) ? result : []
      setSteps(
        arr.map((s) => ({
          id: s.id,
          step_index: s.step_index,
          step_type: s.step_type,
          name: s.name,
          config: (() => {
            try {
              return s.config ? JSON.parse(s.config) : {}
            } catch (e) {
              return {}
            }
          })(),
          output_var: s.output_var || '',
          next_on_success: s.next_on_success ?? 0,
          next_on_failure: s.next_on_failure ?? -1,
        }))
      )
      setSelectedIdx(arr.length > 0 ? 0 : -1)
    } catch (e) {
      console.error('加载步骤失败:', e)
      setSteps([])
    } finally {
      setLoading(false)
    }
  }, [task])

  useEffect(() => {
    if (isOpen) loadSteps()
  }, [isOpen, loadSteps])

  if (!isOpen) return null

  const addStep = () => {
    const newStep = {
      step_type: 'agent',
      name: `步骤 ${steps.length + 1}`,
      config: { agent_name: 'web', prompt: '' },
      output_var: '',
      next_on_success: 0,
      next_on_failure: -1,
    }
    const next = [...steps, newStep]
    setSteps(next)
    setSelectedIdx(next.length - 1)
  }

  const updateStep = (idx, patch) => {
    const next = [...steps]
    next[idx] = { ...next[idx], ...patch }
    setSteps(next)
  }

  const updateStepConfig = (idx, patch) => {
    const next = [...steps]
    next[idx] = { ...next[idx], config: { ...next[idx].config, ...patch } }
    setSteps(next)
  }

  const deleteStep = (idx) => {
    const next = steps.filter((_, i) => i !== idx)
    setSteps(next)
    setSelectedIdx(Math.max(0, Math.min(selectedIdx, next.length - 1)))
    if (next.length === 0) setSelectedIdx(-1)
  }

  const handleDragStart = (idx) => setDragIdx(idx)
  const handleDragOver = (e) => e.preventDefault()
  const handleDrop = (idx) => {
    if (dragIdx === -1 || dragIdx === idx) return
    const next = [...steps]
    const [moved] = next.splice(dragIdx, 1)
    next.splice(idx, 0, moved)
    setSteps(next)
    setDragIdx(-1)
  }

  const moveStep = (idx, dir) => {
    const target = idx + dir
    if (target < 0 || target >= steps.length) return
    const next = [...steps]
    const [moved] = next.splice(idx, 1)
    next.splice(target, 0, moved)
    setSteps(next)
    setSelectedIdx(target)
  }

  const handleSave = async () => {
    if (!task || !hasBackend()) return
    setSaving(true)
    try {
      const payload = steps.map((s, i) => ({
        step_index: i,
        step_type: s.step_type,
        name: s.name || '',
        config: JSON.stringify(s.config || {}),
        output_var: s.output_var || '',
        next_on_success: s.next_on_success ?? 0,
        next_on_failure: s.next_on_failure ?? -1,
      }))
      await window.go.main.App.SaveAutomationSteps(task.id, JSON.stringify(payload))
      onClose()
    } catch (e) {
      alert('保存流程失败: ' + (e.message || e))
    } finally {
      setSaving(false)
    }
  }

  const selected = selectedIdx >= 0 ? steps[selectedIdx] : null
  // 前序步骤的 output_var 列表（供插入变量）
  const availableVars = steps
    .slice(0, selectedIdx)
    .filter((s) => s.output_var)
    .map((s) => ({ var: s.output_var, label: s.name }))

  return (
    <div className="fixed inset-0 z-50 bg-black/50 flex items-center justify-center p-4 backdrop-blur-sm">
      <div className="bg-surface border border-border rounded-xl w-full max-w-5xl max-h-[90vh] flex flex-col overflow-hidden animate-fade-in">
        {/* 头部 */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-border bg-bg">
          <div>
            <h3 className="font-semibold text-text flex items-center gap-2">
              <Workflow className="w-4 h-4 text-violet-400" />
              编辑流程步骤
            </h3>
            <p className="text-xs text-text-muted mt-0.5">{task?.name}</p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={addStep}
              className="flex items-center gap-1 px-3 py-1.5 bg-primary-600 hover:bg-primary-500 text-white text-sm rounded-lg transition-colors"
            >
              <Plus className="w-4 h-4" />
              添加步骤
            </button>
            <button
              onClick={onClose}
              className="p-1.5 text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
            >
              <X className="w-4 h-4" />
            </button>
          </div>
        </div>

        {/* 内容 */}
        <div className="flex-1 flex overflow-hidden">
          {/* 左侧步骤列表 */}
          <div className="w-1/2 border-r border-border overflow-y-auto p-3">
            {loading ? (
              <div className="flex items-center justify-center py-12 text-text-subtle text-sm">
                <Loader2 className="w-4 h-4 animate-spin mr-2" />
                加载中...
              </div>
            ) : steps.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-12 text-text-subtle">
                <Workflow className="w-10 h-10 mb-3 opacity-50" />
                <p className="text-sm">暂无步骤</p>
                <p className="text-xs mt-1">点击「添加步骤」开始编排</p>
              </div>
            ) : (
              <div className="space-y-2">
                {steps.map((s, idx) => {
                  const sInfo = getStepTypeInfo(s.step_type)
                  const SIcon = sInfo.icon
                  const active = selectedIdx === idx
                  return (
                    <div
                      key={idx}
                      draggable
                      onDragStart={() => handleDragStart(idx)}
                      onDragOver={handleDragOver}
                      onDrop={() => handleDrop(idx)}
                      onClick={() => setSelectedIdx(idx)}
                      className={`bg-bg border rounded-lg p-3 cursor-pointer transition-all ${
                        active
                          ? 'border-primary-500 ring-1 ring-primary-500/20'
                          : 'border-border hover:border-border-strong'
                      } ${dragIdx === idx ? 'opacity-50' : ''}`}
                    >
                      <div className="flex items-center justify-between">
                        <div className="flex items-center gap-2 min-w-0">
                          <span className="w-6 h-6 flex items-center justify-center bg-primary-500/10 text-primary-400 text-xs font-medium rounded-full flex-shrink-0">
                            {idx + 1}
                          </span>
                          <SIcon className="w-4 h-4 text-text-muted flex-shrink-0" />
                          <span className="text-sm font-medium text-text truncate">{s.name}</span>
                          {s.output_var && (
                            <span className="text-xs text-text-subtle font-mono truncate">
                              → {s.output_var}
                            </span>
                          )}
                        </div>
                        <div className="flex items-center gap-0.5 flex-shrink-0" onClick={(e) => e.stopPropagation()}>
                          <button
                            onClick={() => moveStep(idx, -1)}
                            disabled={idx === 0}
                            className="p-1 text-text-muted hover:text-text disabled:opacity-30 disabled:cursor-not-allowed"
                            title="上移"
                          >
                            <ChevronDown className="w-3.5 h-3.5 rotate-180" />
                          </button>
                          <button
                            onClick={() => moveStep(idx, 1)}
                            disabled={idx === steps.length - 1}
                            className="p-1 text-text-muted hover:text-text disabled:opacity-30 disabled:cursor-not-allowed"
                            title="下移"
                          >
                            <ChevronDown className="w-3.5 h-3.5" />
                          </button>
                          <button
                            onClick={() => deleteStep(idx)}
                            className="p-1 text-text-muted hover:text-danger-400 hover:bg-danger-500/10 rounded transition-colors"
                            title="删除"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>

          {/* 右侧配置面板 */}
          <div className="w-1/2 overflow-y-auto p-4">
            {!selected ? (
              <div className="flex flex-col items-center justify-center h-full text-text-subtle text-sm">
                <Settings className="w-8 h-8 mb-2 opacity-50" />
                <p>选择左侧步骤查看配置</p>
              </div>
            ) : (
              <StepConfigPanel
                step={selected}
                onChange={(patch) => updateStep(selectedIdx, patch)}
                onConfigChange={(patch) => updateStepConfig(selectedIdx, patch)}
                availableVars={availableVars}
              />
            )}
          </div>
        </div>

        {/* 底部 */}
        <div className="flex justify-end gap-2 px-4 py-3 border-t border-border">
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm text-text-muted hover:text-text hover:bg-surface-subtle rounded-lg transition-colors"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="px-4 py-2 bg-primary-600 hover:bg-primary-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors flex items-center gap-2"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            保存流程
          </button>
        </div>
      </div>
    </div>
  )
}

function StepConfigPanel({ step, onChange, onConfigChange, availableVars }) {
  const labelCls = 'block text-sm font-medium text-text mb-1.5'
  const inputCls =
    'w-full px-3 py-2.5 bg-bg border border-border rounded-lg text-sm focus:outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 transition-all'
  const cfg = step.config || {}

  // 插入变量到文本字段
  const insertVar = (fieldName, varName) => {
    const cur = cfg[fieldName] || ''
    onConfigChange({ [fieldName]: cur + `{{${varName}}}` })
  }

  return (
    <div className="space-y-4">
      <div>
        <label className={labelCls}>步骤名称</label>
        <input
          type="text"
          value={step.name || ''}
          onChange={(e) => onChange({ name: e.target.value })}
          className={inputCls}
        />
      </div>

      <div>
        <label className={labelCls}>步骤类型</label>
        <div className="grid grid-cols-1 gap-1.5">
          {STEP_TYPES.map((t) => {
            const Icon = t.icon
            const active = step.step_type === t.id
            return (
              <button
                key={t.id}
                type="button"
                onClick={() => {
                  onChange({ step_type: t.id })
                  // 切换类型时重置 config 默认值
                  const defaults = {
                    agent: { agent_name: 'web', prompt: '' },
                    search: { query: '', need_summary: true },
                    condition: { source_var: '', operator: 'contains', compare_value: '' },
                    save_file: { content_var: '', format: 'markdown', file_path: '', file_name: '' },
                    notify: { content: '', level: 'normal' },
                  }
                  onConfigChange(defaults[t.id] || {})
                }}
                className={`flex items-center gap-2 p-2 rounded-lg text-left text-sm transition-colors border ${
                  active
                    ? 'bg-primary-500/10 border-primary-500'
                    : 'bg-bg border-border hover:border-border-strong'
                }`}
              >
                <Icon className="w-4 h-4" />
                <div className="flex-1 min-w-0">
                  <div className="text-text">{t.label}</div>
                  <div className="text-xs text-text-muted">{t.desc}</div>
                </div>
              </button>
            )
          })}
        </div>
      </div>

      {/* 输出变量名（agent / search 有） */}
      {(step.step_type === 'agent' || step.step_type === 'search') && (
        <div>
          <label className={labelCls}>输出变量名</label>
          <input
            type="text"
            value={step.output_var || ''}
            onChange={(e) => onChange({ output_var: e.target.value })}
            placeholder="例如：search_result"
            className={inputCls + ' font-mono'}
          />
          <p className="text-xs text-text-subtle mt-1">后续步骤可通过 {'{{变量名}}'} 引用</p>
        </div>
      )}

      {/* agent 配置 */}
      {step.step_type === 'agent' && (
        <>
          <div>
            <label className={labelCls}>选择 Agent</label>
            <select
              value={cfg.agent_name || 'web'}
              onChange={(e) => onConfigChange({ agent_name: e.target.value })}
              className={inputCls}
            >
              {AGENTS.map((a) => (
                <option key={a.value} value={a.value}>
                  {a.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-sm font-medium text-text">提示词</label>
              <VarInsertButton
                availableVars={availableVars}
                onInsert={(v) => insertVar('prompt', v)}
              />
            </div>
            <textarea
              value={cfg.prompt || ''}
              onChange={(e) => onConfigChange({ prompt: e.target.value })}
              placeholder="支持 {{date}} {{time}} {{变量名}}"
              rows={4}
              className={inputCls + ' resize-none font-mono'}
            />
          </div>
        </>
      )}

      {/* search 配置 */}
      {step.step_type === 'search' && (
        <>
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-sm font-medium text-text">搜索关键词</label>
              <VarInsertButton
                availableVars={availableVars}
                onInsert={(v) => insertVar('query', v)}
              />
            </div>
            <input
              type="text"
              value={cfg.query || ''}
              onChange={(e) => onConfigChange({ query: e.target.value })}
              placeholder="支持 {{变量名}}"
              className={inputCls}
            />
          </div>
          <div className="flex items-center gap-2 p-3 bg-surface-subtle rounded-lg">
            <button
              type="button"
              onClick={() => onConfigChange({ need_summary: !cfg.need_summary })}
              className={`relative inline-flex h-6 w-11 items-center rounded-full transition-colors ${
                cfg.need_summary ? 'bg-primary-500' : 'bg-border-strong'
              }`}
            >
              <span
                className={`inline-block h-4 w-4 transform rounded-full bg-white transition-transform ${
                  cfg.need_summary ? 'translate-x-6' : 'translate-x-1'
                }`}
              />
            </button>
            <label className="text-sm text-text">AI 总结搜索结果</label>
          </div>
        </>
      )}

      {/* condition 配置 */}
      {step.step_type === 'condition' && (
        <>
          <div>
            <label className={labelCls}>数据来源变量</label>
            <select
              value={cfg.source_var || ''}
              onChange={(e) => onConfigChange({ source_var: e.target.value })}
              className={inputCls}
            >
              <option value="">选择前序步骤变量...</option>
              {availableVars.map((v) => (
                <option key={v.var} value={v.var}>
                  {v.label} ({v.var})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={labelCls}>判断方式</label>
            <select
              value={cfg.operator || 'contains'}
              onChange={(e) => onConfigChange({ operator: e.target.value })}
              className={inputCls}
            >
              <option value="contains">包含 (contains)</option>
              <option value="not_contains">不包含 (not_contains)</option>
              <option value="equals">等于 (equals)</option>
              <option value="is_empty">为空 (is_empty)</option>
              <option value="not_empty">非空 (not_empty)</option>
            </select>
          </div>
          <div>
            <label className={labelCls}>比较值</label>
            <input
              type="text"
              value={cfg.compare_value || ''}
              onChange={(e) => onConfigChange({ compare_value: e.target.value })}
              className={inputCls}
            />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={labelCls}>条件成立跳转</label>
              <select
                value={step.next_on_success ?? 0}
                onChange={(e) => onChange({ next_on_success: Number(e.target.value) })}
                className={inputCls}
              >
                <option value={0}>执行下一步</option>
                <option value={-1}>结束流程</option>
              </select>
            </div>
            <div>
              <label className={labelCls}>条件不成立跳转</label>
              <select
                value={step.next_on_failure ?? -1}
                onChange={(e) => onChange({ next_on_failure: Number(e.target.value) })}
                className={inputCls}
              >
                <option value={0}>执行下一步</option>
                <option value={-1}>结束流程</option>
                <option value={-2}>重试本步</option>
              </select>
            </div>
          </div>
        </>
      )}

      {/* save_file 配置 */}
      {step.step_type === 'save_file' && (
        <>
          <div>
            <label className={labelCls}>内容来源变量</label>
            <select
              value={cfg.content_var || ''}
              onChange={(e) => onConfigChange({ content_var: e.target.value })}
              className={inputCls}
            >
              <option value="">选择前序步骤变量...</option>
              {availableVars.map((v) => (
                <option key={v.var} value={v.var}>
                  {v.label} ({v.var})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className={labelCls}>文件格式</label>
            <select
              value={cfg.format || 'markdown'}
              onChange={(e) => onConfigChange({ format: e.target.value })}
              className={inputCls}
            >
              <option value="markdown">Markdown</option>
              <option value="text">纯文本</option>
              <option value="json">JSON</option>
            </select>
          </div>
          <div>
            <label className={labelCls}>保存目录</label>
            <input
              type="text"
              value={cfg.file_path || ''}
              onChange={(e) => onConfigChange({ file_path: e.target.value })}
              placeholder="例如：D:\\Reports"
              className={inputCls + ' font-mono'}
            />
          </div>
          <div>
            <label className={labelCls}>文件名</label>
            <input
              type="text"
              value={cfg.file_name || ''}
              onChange={(e) => onConfigChange({ file_name: e.target.value })}
              placeholder="支持 {{date}} {{变量名}}"
              className={inputCls + ' font-mono'}
            />
          </div>
        </>
      )}

      {/* notify 配置 */}
      {step.step_type === 'notify' && (
        <>
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label className="text-sm font-medium text-text">通知内容</label>
              <VarInsertButton
                availableVars={availableVars}
                onInsert={(v) => insertVar('content', v)}
              />
            </div>
            <textarea
              value={cfg.content || ''}
              onChange={(e) => onConfigChange({ content: e.target.value })}
              placeholder="支持 {{变量名}}"
              rows={3}
              className={inputCls + ' resize-none'}
            />
          </div>
          <div>
            <label className={labelCls}>通知级别</label>
            <select
              value={cfg.level || 'normal'}
              onChange={(e) => onConfigChange({ level: e.target.value })}
              className={inputCls}
            >
              <option value="normal">普通</option>
              <option value="important">重要</option>
            </select>
          </div>
        </>
      )}
    </div>
  )
}

// 插入变量按钮
function VarInsertButton({ availableVars, onInsert }) {
  const [open, setOpen] = useState(false)
  if (availableVars.length === 0) return null
  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen(!open)}
        className="text-xs text-primary-400 hover:text-primary-300 flex items-center gap-1"
      >
        <Plus className="w-3 h-3" />
        插入变量
      </button>
      {open && (
        <>
          <div className="fixed inset-0 z-10" onClick={() => setOpen(false)} />
          <div className="absolute right-0 mt-1 w-44 bg-surface border border-border rounded-lg shadow-xl z-20 py-1">
            {availableVars.map((v) => (
              <button
                key={v.var}
                type="button"
                onClick={() => {
                  onInsert(v.var)
                  setOpen(false)
                }}
                className="w-full text-left px-3 py-1.5 text-xs hover:bg-surface-subtle"
              >
                <span className="text-text">{v.label}</span>
                <span className="text-text-muted ml-2 font-mono">{v.var}</span>
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

// ---------- 主页面 ----------
function AutomationPage() {
  const [tasks, setTasks] = useState([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(null)
  const [expandedId, setExpandedId] = useState(null)
  const [executions, setExecutions] = useState([])
  const [loadingExecutions, setLoadingExecutions] = useState(false)

  const [showWizard, setShowWizard] = useState(false)
  const [editingTask, setEditingTask] = useState(null)
  const [showWorkflowEditor, setShowWorkflowEditor] = useState(false)
  const [workflowTask, setWorkflowTask] = useState(null)

  const loadTasks = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      if (hasBackend()) {
        const result = await window.go.main.App.GetAutomationTasks('')
        setTasks(Array.isArray(result) ? result : [])
      }
    } catch (err) {
      console.error('加载任务失败:', err)
      setError('加载任务失败: ' + (err.message || err))
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadTasks()
  }, [loadTasks])

  const loadExecutions = useCallback(async (taskId) => {
    setLoadingExecutions(true)
    try {
      if (hasBackend()) {
        const result = await window.go.main.App.GetAutomationExecutions(taskId)
        setExecutions(Array.isArray(result) ? result : [])
      }
    } catch (err) {
      console.error('加载执行记录失败:', err)
      setExecutions([])
    } finally {
      setLoadingExecutions(false)
    }
  }, [])

  const handleToggleExpand = (taskId) => {
    if (expandedId === taskId) {
      setExpandedId(null)
      setExecutions([])
    } else {
      setExpandedId(taskId)
      loadExecutions(taskId)
    }
  }

  const handleCreateOrUpdate = async (data) => {
    if (!hasBackend()) return
    if (editingTask) {
      await window.go.main.App.UpdateAutomationTask(
        editingTask.id,
        data.name,
        data.description,
        data.taskType,
        data.config,
        data.scheduleType,
        data.scheduleConfig,
        data.enabled,
        data.slashCommand || ''
      )
    } else {
      if (data.templateID) {
        await window.go.main.App.CreateTaskFromTemplate(
          data.templateID,
          data.name,
          data.description,
          data.scheduleType,
          data.scheduleConfig,
          data.slashCommand || ''
        )
      } else {
        await window.go.main.App.CreateAutomationTask(
          data.name,
          data.description,
          data.taskType,
          data.config,
          data.scheduleType,
          data.scheduleConfig,
          data.enabled,
          data.slashCommand || ''
        )
      }
    }
    setEditingTask(null)
    loadTasks()
  }

  const handleDelete = async (id) => {
    if (!window.confirm('确定要删除这个任务吗？此操作无法撤销。')) return
    setError(null)
    try {
      if (hasBackend()) {
        await window.go.main.App.DeleteAutomationTask(id)
        if (expandedId === id) setExpandedId(null)
        loadTasks()
      }
    } catch (err) {
      setError('删除任务失败: ' + (err.message || err))
    }
  }

  const handleToggle = async (id, enabled) => {
    setError(null)
    try {
      if (hasBackend()) {
        await window.go.main.App.ToggleAutomationTask(id, enabled)
        loadTasks()
      }
    } catch (err) {
      setError('操作失败: ' + (err.message || err))
    }
  }

  const handleRunNow = async (id) => {
    setError(null)
    try {
      if (hasBackend()) {
        await window.go.main.App.RunAutomationTaskNow(id)
        // 如果当前展开了这个任务，刷新执行记录
        if (expandedId === id) {
          setTimeout(() => loadExecutions(id), 800)
        }
      }
    } catch (err) {
      setError('触发任务失败: ' + (err.message || err))
    }
  }

  const openEdit = (task) => {
    setEditingTask(task)
    setShowWizard(true)
  }

  const openWorkflowEditor = (task) => {
    setWorkflowTask(task)
    setShowWorkflowEditor(true)
  }

  // 统计
  const runningCount = tasks.filter((t) => t.enabled).length
  const pausedCount = tasks.length - runningCount
  const todayStr = new Date().toDateString()
  const todayCount = tasks.filter(
    (t) => t.last_run_at && t.last_run_at !== '0001-01-01T00:00:00Z' && new Date(t.last_run_at).toDateString() === todayStr
  ).length

  // 过滤
  const filteredTasks = tasks

  return (
    <div className="h-full flex flex-col bg-bg">
      {/* 顶部 */}
      <div className="px-4 py-4 border-b border-border bg-surface/80 backdrop-blur-sm">
        <div className="flex items-center justify-between">
          <div>
            <h2 className="text-lg font-semibold text-text flex items-center gap-2">
              <Zap className="w-5 h-5 text-primary-400" />
              自动化
            </h2>
            <p className="text-sm text-text-subtle mt-0.5">统一管理周期任务与流程编排</p>
          </div>
          <button
            onClick={() => {
              setEditingTask(null)
              setShowWizard(true)
            }}
            className="flex items-center gap-2 px-3 py-2 bg-primary-600 hover:bg-primary-500 text-white rounded-lg text-sm font-medium transition-all hover:shadow-lg hover:shadow-primary-500/25"
          >
            <Plus className="w-4 h-4" />
            新建任务
          </button>
        </div>

        {/* 统计栏 */}
        <div className="flex items-center gap-2 mt-3">
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-surface-subtle rounded-lg">
            <div className="w-2 h-2 rounded-full bg-success-500" />
            <span className="text-xs text-text-muted">运行中: {runningCount}</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-surface-subtle rounded-lg">
            <div className="w-2 h-2 rounded-full bg-warning-500" />
            <span className="text-xs text-text-muted">已暂停: {pausedCount}</span>
          </div>
          <div className="flex items-center gap-1.5 px-3 py-1.5 bg-surface-subtle rounded-lg">
            <div className="w-2 h-2 rounded-full bg-primary-500" />
            <span className="text-xs text-text-muted">今日执行: {todayCount}</span>
          </div>
        </div>
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="flex items-center gap-2 px-4 py-2 bg-danger-500/10 border-b border-danger-500/20 text-danger-500 text-sm">
          <AlertCircle className="w-4 h-4 flex-shrink-0" />
          <span className="flex-1">{error}</span>
          <button onClick={() => setError(null)} className="p-0.5 hover:bg-danger-500/20 rounded">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* 列表 */}
      <div className="flex-1 overflow-y-auto p-4">
        {loading ? (
          <div className="flex flex-col items-center justify-center h-full text-text-subtle">
            <div className="w-10 h-10 border-4 border-border border-t-primary-500 rounded-full animate-spin mb-3" />
            <p className="text-sm">加载中...</p>
          </div>
        ) : filteredTasks.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-text-subtle">
            <div className="w-16 h-16 rounded-full bg-surface-subtle flex items-center justify-center mb-4">
              <Zap className="w-8 h-8 opacity-50" />
            </div>
            <p className="text-text-muted">暂无自动化任务</p>
            <p className="text-sm mt-1">点击上方按钮创建第一个任务</p>
            <div className="mt-4 flex items-center gap-2 text-xs text-text-subtle">
              <Info className="w-3.5 h-3.5" />
              <span>支持对话/搜索/报告/备份/提醒/监控/打卡/复习/清理/流程</span>
            </div>
          </div>
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {filteredTasks.map((task) => (
              <TaskCard
                key={task.id}
                task={task}
                expanded={expandedId === task.id}
                executions={expandedId === task.id ? executions : []}
                loadingExecutions={loadingExecutions}
                onToggleExpand={handleToggleExpand}
                onEdit={openEdit}
                onDelete={handleDelete}
                onToggle={handleToggle}
                onRunNow={handleRunNow}
                onEditWorkflow={openWorkflowEditor}
              />
            ))}
          </div>
        )}
      </div>

      {/* 新建/编辑任务弹窗 */}
      <TaskWizard
        isOpen={showWizard}
        onClose={() => {
          setShowWizard(false)
          setEditingTask(null)
        }}
        onSubmit={handleCreateOrUpdate}
        editingTask={editingTask}
      />

      {/* 流程编辑器 */}
      <WorkflowEditor
        isOpen={showWorkflowEditor}
        onClose={() => {
          setShowWorkflowEditor(false)
          setWorkflowTask(null)
        }}
        task={workflowTask}
      />
    </div>
  )
}

export default AutomationPage
