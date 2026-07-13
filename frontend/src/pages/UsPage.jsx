import React, { useState, useEffect, useCallback } from 'react'
import { Calendar, Star, TrendingUp, BookOpen, RotateCcw, Loader2, AlertCircle, Plus, X, Trash2, ChevronDown, ChevronUp } from 'lucide-react'

// 格式化后端 time.Time 为 YYYY-MM-DD
const formatDate = (raw) => {
  if (!raw) return ''
  const str = typeof raw === 'string' ? raw : String(raw)
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
    return typeof window !== 'undefined' && window.go && window.go.main && window.go.main && window.go.main.App
  } catch (e) {
    return false
  }
}

function UsPage() {
  const [activeSection, setActiveSection] = useState('timeline')
  const [showReflection, setShowReflection] = useState(false)
  const [highlights, setHighlights] = useState([])
  const [reflections, setReflections] = useState([])
  const [memories, setMemories] = useState([])
  const [reflection, setReflection] = useState(null)
  const [loading, setLoading] = useState(false)
  const [reflectionLoading, setReflectionLoading] = useState(false)
  const [error, setError] = useState(null)
  const [reflectionError, setReflectionError] = useState(null)
  // 添加回忆表单
  const [showAddForm, setShowAddForm] = useState(false)
  const [newHighlight, setNewHighlight] = useState({ title: '', description: '', date: '' })
  const [addLoading, setAddLoading] = useState(false)
  const [addError, setAddError] = useState(null)
  // 复盘周期选择
  const [reflectionPeriod, setReflectionPeriod] = useState('week')
  // 复盘历史展开
  const [expandedReflection, setExpandedReflection] = useState(null)
  // 删除回忆
  const [deleteLoadingId, setDeleteLoadingId] = useState(null)

  const backendAvailable = hasBackend()

  const sections = [
    { id: 'timeline', label: '时间线', icon: Calendar },
    { id: 'highlights', label: '回忆', icon: Star },
    { id: 'growth', label: '成长', icon: TrendingUp },
  ]

  // 加载高光回忆
  const loadHighlights = useCallback(async () => {
    if (!hasBackend()) return
    setLoading(true)
    setError(null)
    try {
      const result = await window.go.main.App.GetHighlights()
      setHighlights(Array.isArray(result) ? result : [])
    } catch (err) {
      console.error('GetHighlights failed:', err)
      setError(err?.message || String(err) || '获取高光回忆失败')
      setHighlights([])
    } finally {
      setLoading(false)
    }
  }, [])

  // 加载记忆（用于时间线和成长统计）
  const loadMemories = useCallback(async () => {
    if (!hasBackend()) return
    try {
      const result = await window.go.main.App.GetMemories('')
      setMemories(Array.isArray(result) ? result : [])
    } catch (err) {
      console.error('GetMemories failed:', err)
      if (!error) {
        setError(err?.message || String(err) || '获取记忆失败')
      }
    }
  }, [error])

  // 加载复盘历史
  const loadReflections = useCallback(async () => {
    if (!hasBackend()) return
    try {
      const result = await window.go.main.App.GetReflections()
      setReflections(Array.isArray(result) ? result : [])
    } catch (err) {
      console.error('GetReflections failed:', err)
    }
  }, [])

  useEffect(() => {
    if (backendAvailable) {
      loadHighlights()
      loadMemories()
      loadReflections()
    }
  }, [backendAvailable, loadHighlights, loadMemories, loadReflections])

  // 添加回忆
  const handleAddHighlight = async () => {
    if (!hasBackend()) {
      setAddError('后端不可用')
      return
    }
    if (!newHighlight.title.trim()) {
      setAddError('请填写标题')
      return
    }
    setAddLoading(true)
    setAddError(null)
    try {
      await window.go.main.App.AddHighlight(
        newHighlight.title.trim(),
        newHighlight.description.trim(),
        newHighlight.date || formatDate(new Date().toISOString())
      )
      setNewHighlight({ title: '', description: '', date: '' })
      setShowAddForm(false)
      await loadHighlights()
    } catch (err) {
      console.error('AddHighlight failed:', err)
      setAddError(err?.message || String(err) || '添加回忆失败')
    } finally {
      setAddLoading(false)
    }
  }

  // 生成复盘
  const handleGenerateReflection = async () => {
    if (!hasBackend()) {
      setReflectionError('后端不可用')
      setShowReflection(true)
      return
    }
    setShowReflection(true)
    setReflectionLoading(true)
    setReflectionError(null)
    setReflection(null)
    try {
      const result = await window.go.main.App.GenerateReflection(reflectionPeriod)
      setReflection(result || null)
      // 复盘后刷新历史列表
      loadReflections()
    } catch (err) {
      console.error('GenerateReflection failed:', err)
      setReflectionError(err?.message || String(err) || '生成复盘失败')
    } finally {
      setReflectionLoading(false)
    }
  }

  // 删除回忆
  const handleDeleteHighlight = async (id) => {
    if (!confirm('确定要删除这条回忆吗？')) return
    setDeleteLoadingId(id)
    try {
      await window.go.main.App.DeleteHighlight(id)
      await loadHighlights()
    } catch (err) {
      console.error('DeleteHighlight failed:', err)
    } finally {
      setDeleteLoadingId(null)
    }
  }

  // 构建时间线：合并高光回忆与记忆，按日期降序
  const timeline = (() => {
    const events = []
    highlights.forEach((h) => {
      events.push({
        date: formatDate(h.date || h.created_at),
        title: h.title || '高光回忆',
        description: h.description || '',
        source: 'highlight',
      })
    })
    memories.forEach((m) => {
      const date = formatDate(m.created_at || m.date || m.timestamp)
      const content = m.content || m.summary || m.title || ''
      if (!content) return
      events.push({
        date,
        title: m.title || (m.type ? m.type : '记忆'),
        description: content,
        source: 'memory',
      })
    })
    // 去重并按日期降序排序
    return events
      .filter((e) => e.date)
      .sort((a, b) => (a.date < b.date ? 1 : a.date > b.date ? -1 : 0))
  })()

  // 成长统计：从记忆中统计各类型数量
  const growthStats = (() => {
    const stats = {}
    memories.forEach((m) => {
      const type = m.type || m.memory_type || '其他'
      stats[type] = (stats[type] || 0) + 1
    })
    const total = memories.length || 1
    const entries = Object.entries(stats)
    if (entries.length === 0) {
      // 没有记忆时回退到示例
      return [
        { label: 'Go 语言', value: 20 },
        { label: '项目规划', value: 35 },
        { label: '情绪管理', value: 50 },
      ]
    }
    // 按数量倒序
    entries.sort((a, b) => b[1] - a[1])
    return entries.slice(0, 5).map(([label, count]) => ({
      label,
      value: Math.round((count / total) * 100),
    }))
  })()

  // 空状态：后端不可用
  if (!backendAvailable) {
    return (
      <div className="max-w-3xl mx-auto py-4 px-4">
        <div className="flex items-center justify-between mb-4">
          <h2 className="text-lg font-semibold text-text">我们</h2>
          <button
            disabled
            className="flex items-center gap-1 px-3 py-1.5 bg-bg-hover text-text-subtle rounded-lg text-sm cursor-not-allowed"
          >
            <RotateCcw className="w-4 h-4" />
            复盘
          </button>
        </div>
        <div className="flex flex-col items-center justify-center py-16 text-center">
          <AlertCircle className="w-10 h-10 text-text-subtle mb-3" />
          <p className="text-sm text-text-muted">后端未连接</p>
          <p className="text-xs text-text-subtle mt-1">请通过 Wails 应用启动后使用此页面功能</p>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-3xl mx-auto py-4 px-4">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold text-text">我们</h2>
        <button
          onClick={handleGenerateReflection}
          disabled={reflectionLoading}
          className="flex items-center gap-1 px-3 py-1.5 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white rounded-lg text-sm transition-colors"
        >
          {reflectionLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <RotateCcw className="w-4 h-4" />}
          复盘
        </button>
      </div>

      {/* 分区标签 */}
      <div className="flex gap-2 mb-4 overflow-x-auto">
        {sections.map((section) => {
          const Icon = section.icon
          return (
            <button
              key={section.id}
              onClick={() => setActiveSection(section.id)}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-sm transition-colors whitespace-nowrap ${
                activeSection === section.id
                  ? 'bg-bg-hover text-text'
                  : 'text-text-subtle hover:text-text-muted hover:bg-surface-subtle/50'
              }`}
            >
              <Icon className="w-4 h-4" />
              {section.label}
            </button>
          )
        })}
      </div>

      {/* 错误提示 */}
      {error && (
        <div className="mb-4 flex items-start gap-2 bg-red-900/30 border border-red-800/50 rounded-lg px-3 py-2 text-sm text-red-300">
          <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <div className="flex-1">{error}</div>
        </div>
      )}

      {/* 时间线 */}
      {activeSection === 'timeline' && (
        <div className="space-y-0">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <Loader2 className="w-6 h-6 text-text-subtle animate-spin" />
            </div>
          ) : timeline.length === 0 ? (
            <div className="flex flex-col items-center justify-center py-12 text-center">
              <Calendar className="w-8 h-8 text-text-subtle mb-2" />
              <p className="text-sm text-text-muted">还没有时间线记录</p>
              <p className="text-xs text-text-subtle mt-1">添加回忆或与伙伴对话后将出现在这里</p>
            </div>
          ) : (
            timeline.map((event, idx) => (
              <div key={idx} className="flex gap-4 group">
                <div className="flex flex-col items-center">
                  <div className={`w-2.5 h-2.5 rounded-full ${event.source === 'highlight' ? 'bg-amber-400' : 'bg-primary-500'} ring-4 ring-bg`} />
                  {idx < timeline.length - 1 && (
                    <div className="w-px h-full bg-surface-subtle group-hover:bg-bg-hover transition-colors" />
                  )}
                </div>
                <div className="pb-6">
                  <div className="text-xs text-text-subtle mb-0.5">{event.date}</div>
                  <div className="text-sm font-medium text-text">{event.title}</div>
                  <div className="text-sm text-text-subtle mt-0.5">{event.description}</div>
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* 共同回忆 */}
      {activeSection === 'highlights' && (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          {loading ? (
            <div className="col-span-full flex items-center justify-center py-12">
              <Loader2 className="w-6 h-6 text-text-subtle animate-spin" />
            </div>
          ) : (
            <>
              {highlights.map((highlight) => (
                <div
                  key={highlight.id}
                  className="bg-surface/60 border border-border rounded-xl p-4 hover:border-border-strong transition-colors group relative"
                >
                  <div className="flex items-start justify-between mb-2">
                    <Star className={`w-5 h-5 ${highlight.user_marked ? 'text-amber-400' : 'text-text-subtle'}`} />
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-text-subtle">{formatDate(highlight.date)}</span>
                      <button
                        onClick={() => handleDeleteHighlight(highlight.id)}
                        disabled={deleteLoadingId === highlight.id}
                        className="p-1 text-text-subtle hover:text-red-400 transition-colors disabled:opacity-50 opacity-0 group-hover:opacity-100"
                        title="删除回忆"
                      >
                        {deleteLoadingId === highlight.id ? (
                          <Loader2 className="w-3.5 h-3.5 animate-spin" />
                        ) : (
                          <Trash2 className="w-3.5 h-3.5" />
                        )}
                      </button>
                    </div>
                  </div>
                  <h3 className="font-medium text-text mb-1">{highlight.title}</h3>
                  <p className="text-sm text-text-subtle">{highlight.description}</p>
                </div>
              ))}
              <button
                onClick={() => setShowAddForm(true)}
                className="border border-dashed border-border rounded-xl p-4 flex items-center justify-center text-text-subtle hover:text-text-muted hover:border-border-strong transition-colors min-h-[100px]"
              >
                <span className="text-sm">+ 添加回忆</span>
              </button>
              {highlights.length === 0 && (
                <div className="col-span-full text-center py-8 text-text-subtle">
                  <p className="text-sm">还没有共同回忆</p>
                  <p className="text-xs mt-1">点击下方按钮记录一段值得记住的时光</p>
                </div>
              )}
            </>
          )}
        </div>
      )}

      {/* 成长轨迹 */}
      {activeSection === 'growth' && (
        <div className="space-y-4">
          <div className="bg-surface/60 border border-border rounded-xl p-4">
            <h3 className="text-sm font-medium text-text mb-3">能力变化</h3>
            <div className="space-y-3">
              {growthStats.map((stat, idx) => (
                <div key={idx}>
                  <div className="flex justify-between text-xs text-text-muted mb-1">
                    <span>{stat.label}</span>
                    <span>{stat.value}%</span>
                  </div>
                  <div className="h-2 bg-surface rounded-full overflow-hidden">
                    <div
                      className="h-full bg-primary-500 rounded-full transition-all duration-1000"
                      style={{ width: `${stat.value}%` }}
                    />
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="bg-surface/60 border border-border rounded-xl p-4">
            <h3 className="text-sm font-medium text-text mb-2">关系深度</h3>
            <p className="text-sm text-text-subtle">
              信任度正在稳步建立中。每一次真诚的对话都在加深我们的连接。
            </p>
          </div>
          {/* 复盘历史 */}
          <div className="bg-surface/60 border border-border rounded-xl p-4">
            <h3 className="text-sm font-medium text-text mb-3 flex items-center gap-2">
              <BookOpen className="w-4 h-4 text-primary-400" />
              复盘历史
            </h3>
            {reflections.length === 0 ? (
              <p className="text-xs text-text-subtle">还没有生成过复盘</p>
            ) : (
              <div className="space-y-2">
                {reflections.map((r) => (
                  <div key={r.id} className="bg-surface/50 rounded-lg overflow-hidden">
                    <button
                      onClick={() => setExpandedReflection(expandedReflection === r.id ? null : r.id)}
                      className="w-full flex items-center justify-between p-3 text-left hover:bg-surface-subtle/50 transition-colors"
                    >
                      <div>
                        <div className="text-sm text-text">
                          {formatDate(r.period_start)} ~ {formatDate(r.period_end)}
                        </div>
                        <div className="text-xs text-text-subtle mt-0.5 line-clamp-1">
                          {(r.growth_analysis || '').split('\n')[0] || '点击查看详情'}
                        </div>
                      </div>
                      {expandedReflection === r.id ? (
                        <ChevronUp className="w-4 h-4 text-text-subtle" />
                      ) : (
                        <ChevronDown className="w-4 h-4 text-text-subtle" />
                      )}
                    </button>
                    {expandedReflection === r.id && (
                      <div className="px-3 pb-3 space-y-2 border-t border-border/50 pt-2">
                        {r.growth_analysis && (
                          <div>
                            <div className="text-xs text-primary-400 mb-0.5">成长</div>
                            <p className="text-xs text-text-muted whitespace-pre-wrap leading-relaxed">{r.growth_analysis}</p>
                          </div>
                        )}
                        {r.relationship_analysis && (
                          <div>
                            <div className="text-xs text-primary-400 mb-0.5">关系</div>
                            <p className="text-xs text-text-muted whitespace-pre-wrap leading-relaxed">{r.relationship_analysis}</p>
                          </div>
                        )}
                        {r.project_review && (
                          <div>
                            <div className="text-xs text-primary-400 mb-0.5">项目</div>
                            <p className="text-xs text-text-muted whitespace-pre-wrap leading-relaxed">{r.project_review}</p>
                          </div>
                        )}
                        {r.observations && (
                          <div>
                            <div className="text-xs text-primary-400 mb-0.5">观察</div>
                            <p className="text-xs text-text-muted whitespace-pre-wrap leading-relaxed">{r.observations}</p>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* 添加回忆弹窗 */}
      {showAddForm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="bg-surface-subtle border border-border rounded-2xl max-w-md w-full animate-slide-up">
            <div className="p-5">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <Star className="w-5 h-5 text-amber-400" />
                  <h3 className="text-lg font-semibold text-text">添加回忆</h3>
                </div>
                <button
                  onClick={() => {
                    setShowAddForm(false)
                    setAddError(null)
                  }}
                  className="text-text-subtle hover:text-text-muted transition-colors"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>
              <div className="space-y-3">
                <div>
                  <label className="block text-xs text-text-muted mb-1">标题</label>
                  <input
                    type="text"
                    value={newHighlight.title}
                    onChange={(e) => setNewHighlight((p) => ({ ...p, title: e.target.value }))}
                    placeholder="给这段回忆起个名字"
                    className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20 transition-all"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-muted mb-1">描述</label>
                  <textarea
                    value={newHighlight.description}
                    onChange={(e) => setNewHighlight((p) => ({ ...p, description: e.target.value }))}
                    placeholder="发生了什么？为什么难忘？"
                    rows={3}
                    className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-text placeholder-text-subtle resize-none focus:outline-none focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20 transition-all"
                  />
                </div>
                <div>
                  <label className="block text-xs text-text-muted mb-1">日期</label>
                  <input
                    type="date"
                    value={newHighlight.date}
                    onChange={(e) => setNewHighlight((p) => ({ ...p, date: e.target.value }))}
                    className="w-full bg-surface border border-border rounded-lg px-3 py-2 text-sm text-text focus:outline-none focus:border-primary-500/50 focus:ring-1 focus:ring-primary-500/20 transition-all"
                  />
                </div>
                {addError && (
                  <div className="flex items-start gap-2 bg-red-900/30 border border-red-800/50 rounded-lg px-3 py-2 text-xs text-red-300">
                    <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                    <div className="flex-1">{addError}</div>
                  </div>
                )}
              </div>
              <div className="mt-6 flex justify-end gap-2">
                <button
                  onClick={() => {
                    setShowAddForm(false)
                    setAddError(null)
                  }}
                  className="px-4 py-2 bg-bg-hover hover:bg-border-strong text-text rounded-lg text-sm transition-colors"
                >
                  取消
                </button>
                <button
                  onClick={handleAddHighlight}
                  disabled={addLoading}
                  className="flex items-center gap-1.5 px-4 py-2 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white rounded-lg text-sm transition-colors"
                >
                  {addLoading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Plus className="w-4 h-4" />}
                  保存
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 复盘弹窗 */}
      {showReflection && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="bg-surface-subtle border border-border rounded-2xl max-w-lg w-full max-h-[80vh] overflow-y-auto animate-slide-up">
            <div className="p-5">
              <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-2">
                  <BookOpen className="w-5 h-5 text-primary-400" />
                  <h3 className="text-lg font-semibold text-text">复盘报告</h3>
                </div>
                <button
                  onClick={() => setShowReflection(false)}
                  className="text-text-subtle hover:text-text-muted transition-colors"
                >
                  <X className="w-5 h-5" />
                </button>
              </div>

              {/* 复盘周期选择 */}
              {!reflectionLoading && (
                <div className="mb-4">
                  <label className="block text-xs text-text-muted mb-1.5">复盘周期</label>
                  <div className="grid grid-cols-3 gap-2">
                    {[
                      { id: 'day', label: '今日' },
                      { id: 'week', label: '本周' },
                      { id: 'month', label: '本月' },
                    ].map((p) => (
                      <button
                        key={p.id}
                        onClick={() => setReflectionPeriod(p.id)}
                        className={`px-3 py-1.5 rounded-lg text-sm transition-colors ${
                          reflectionPeriod === p.id
                            ? 'bg-primary-600 text-white'
                            : 'bg-surface text-text-muted hover:text-text'
                        }`}
                      >
                        {p.label}
                      </button>
                    ))}
                  </div>
                  <button
                    onClick={handleGenerateReflection}
                    disabled={reflectionLoading}
                    className="w-full mt-3 px-3 py-2 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white rounded-lg text-sm transition-colors flex items-center justify-center gap-1.5"
                  >
                    {reflectionLoading ? (
                      <Loader2 className="w-4 h-4 animate-spin" />
                    ) : (
                      <RotateCcw className="w-4 h-4" />
                    )}
                    重新生成
                  </button>
                </div>
              )}

              {reflectionLoading ? (
                <div className="flex flex-col items-center justify-center py-12">
                  <Loader2 className="w-8 h-8 text-primary-400 animate-spin mb-3" />
                  <p className="text-sm text-text-muted">正在生成复盘报告...</p>
                  <p className="text-xs text-text-subtle mt-1">这可能需要一点时间</p>
                </div>
              ) : reflectionError ? (
                <div className="flex items-start gap-2 bg-red-900/30 border border-red-800/50 rounded-lg px-3 py-2 text-sm text-red-300">
                  <AlertCircle className="w-4 h-4 flex-shrink-0 mt-0.5" />
                  <div className="flex-1">{reflectionError}</div>
                </div>
              ) : reflection ? (
                <div className="space-y-4">
                  {reflection.period_start && reflection.period_end && (
                    <div className="text-xs text-text-subtle">
                      区间：{formatDate(reflection.period_start)} ~ {formatDate(reflection.period_end)}
                    </div>
                  )}
                  <div>
                    <div className="text-xs text-primary-400 font-medium mb-1">成长分析</div>
                    <p className="text-sm text-text-muted leading-relaxed whitespace-pre-wrap">
                      {reflection.growth_analysis || '暂无分析内容'}
                    </p>
                  </div>
                  <div>
                    <div className="text-xs text-primary-400 font-medium mb-1">关系分析</div>
                    <p className="text-sm text-text-muted leading-relaxed whitespace-pre-wrap">
                      {reflection.relationship_analysis || '暂无分析内容'}
                    </p>
                  </div>
                  <div>
                    <div className="text-xs text-primary-400 font-medium mb-1">项目复盘</div>
                    <p className="text-sm text-text-muted leading-relaxed whitespace-pre-wrap">
                      {reflection.project_review || '暂无复盘内容'}
                    </p>
                  </div>
                  <div>
                    <div className="text-xs text-primary-400 font-medium mb-1">观察</div>
                    <p className="text-sm text-text-muted leading-relaxed whitespace-pre-wrap">
                      {reflection.observations || '暂无观察内容'}
                    </p>
                  </div>
                </div>
              ) : (
                <div className="flex flex-col items-center justify-center py-12 text-center">
                  <BookOpen className="w-8 h-8 text-text-subtle mb-2" />
                  <p className="text-sm text-text-muted">暂无复盘内容</p>
                </div>
              )}

              <div className="mt-6 flex justify-end">
                <button
                  onClick={() => setShowReflection(false)}
                  className="px-4 py-2 bg-bg-hover hover:bg-border-strong text-text rounded-lg text-sm transition-colors"
                >
                  关闭
                </button>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default UsPage
