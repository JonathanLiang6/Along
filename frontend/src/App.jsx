import React, { useState, useEffect, useCallback, lazy, Suspense } from 'react'
import { Target, Users, Brain, Settings, Loader2, Search, X, Zap, Bot } from 'lucide-react'
import { hasBackend } from './utils/backend'
import ErrorBoundary from './components/ErrorBoundary'
import { LoadingSpinner } from './components/ui'

// 代码分割：每个页面按需加载
const CompanionPage = lazy(() => import('./pages/CompanionPage'))
const PlanPage = lazy(() => import('./pages/PlanPage'))
const AutomationPage = lazy(() => import('./pages/automation/AutomationPage'))
const UsPage = lazy(() => import('./pages/UsPage'))
const MemoryPage = lazy(() => import('./pages/MemoryPage'))
const SettingsPage = lazy(() => import('./pages/SettingsPage'))
const OnboardingPage = lazy(() => import('./pages/OnboardingPage'))

const tabs = [
  { id: 'companion', label: '伙伴', icon: Bot },
  { id: 'plan', label: '计划', icon: Target },
  { id: 'automation', label: '自动化', icon: Zap },
  { id: 'us', label: '我们', icon: Users },
  { id: 'memory', label: '记忆', icon: Brain },
]

// 页面加载中 fallback
function PageFallback() {
  return (
    <div className="flex items-center justify-center h-full min-h-[300px]">
      <div className="flex items-center gap-2 text-text-muted">
        <Loader2 className="w-5 h-5 animate-spin" />
        <span className="text-sm">加载中...</span>
      </div>
    </div>
  )
}

// 【黑屏修复】Go 后端启动中提示。
// 后端 asyncInit（DB / 服务 / 调度器 / 托盘等）还在后台进行时，
// 给用户一个明确的"启动中"状态，而不是空白的"白屏/黑屏"。
// phaseLabel 是后端 GetInitPhase() 返回的中文文案，会随阶段变化实时更新。
//
// 【关键】所有颜色都用 hardcoded 值，不依赖主题变量：
//   旧版用 bg-bg / text-text（来自 --bg / --text CSS 变量），dark 主题下
//   --bg = 9 9 17 几乎纯黑，配合浅色 spinner 很容易被误以为"卡死"。
//   新版用 style 内联浅色背景 + 深色文字，无论 dark/light 都始终清晰可见。
function BackendBooting({ label = '后台服务初始化中，请稍候…', phaseLabel = '' }) {
  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: '#f8fafc',
        color: '#0f172a',
        fontFamily: "'Inter', 'Noto Sans SC', system-ui, -apple-system, sans-serif",
        zIndex: 10000,
      }}
    >
      <div style={{ textAlign: 'center', maxWidth: 360, padding: 24 }}>
        <div
          aria-hidden="true"
          style={{
            width: 40,
            height: 40,
            border: '3px solid #cbd5e1',
            borderTopColor: '#6366f1',
            borderRadius: '50%',
            animation: 'boot-spin 0.9s linear infinite',
            margin: '0 auto 16px',
          }}
        />
        <style>{`@keyframes boot-spin { to { transform: rotate(360deg); } }`}</style>
        <div style={{ fontSize: 18, fontWeight: 600, margin: '0 0 6px' }}>Along 正在启动</div>
        <div style={{ fontSize: 13, color: '#475569' }}>{phaseLabel || label}</div>
      </div>
    </div>
  )
}

// 应用主题
const applyTheme = (theme) => {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.classList.remove('light', 'dark')
  let effective = theme
  if (theme === 'system' || !theme) {
    effective = window.matchMedia('(prefers-color-scheme: dark)').matches
      ? 'dark'
      : 'light'
  }
  root.classList.add(effective)
  root.setAttribute('data-theme', effective)
}

// 搜索弹窗
function SearchModal({ onClose }) {
  const [query, setQuery] = useState('')
  const [results, setResults] = useState([])
  const [loading, setLoading] = useState(false)

  const doSearch = useCallback(async (q) => {
    if (!q.trim()) {
      setResults([])
      return
    }
    setLoading(true)
    try {
      if (!hasBackend()) { setResults([]); return }
      const mems = await window.go.main.App.GetMemories('')
      const filtered = (Array.isArray(mems) ? mems : [])
        .filter((m) => (m.content || '').toLowerCase().includes(q.toLowerCase()))
        .slice(0, 20)
      setResults(filtered)
    } catch (e) {
      setResults([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    const t = setTimeout(() => doSearch(query), 200)
    return () => clearTimeout(t)
  }, [query, doSearch])

  return (
    <div
      className="fixed inset-0 z-50 bg-black/50 flex items-start justify-center pt-20"
      onClick={onClose}
    >
      <div
        className="w-full max-w-lg bg-surface border border-border rounded-xl shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center gap-2 px-4 py-3 border-b border-border">
          <Search className="w-4 h-4 text-text-muted" />
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索记忆内容..."
            autoFocus
            className="flex-1 bg-transparent text-text placeholder-text-subtle text-sm focus:outline-none"
          />
          <button onClick={onClose} className="text-text-muted hover:text-text">
            <X className="w-4 h-4" />
          </button>
        </div>
        <div className="max-h-80 overflow-y-auto p-2">
          {loading ? (
            <div className="flex items-center justify-center py-8 text-text-subtle text-sm">
              <Loader2 className="w-4 h-4 animate-spin mr-2" /> 搜索中...
            </div>
          ) : results.length === 0 ? (
            <div className="text-center py-8 text-text-subtle text-sm">
              {query ? '没有匹配的记忆' : '输入关键词搜索'}
            </div>
          ) : (
            results.map((m) => (
              <div
                key={m.id}
                className="px-3 py-2 rounded-lg hover:bg-surface-subtle cursor-pointer"
              >
                <div className="text-xs text-primary-400 mb-1">{m.type}</div>
                <div className="text-sm text-text line-clamp-2">{m.content}</div>
              </div>
            ))
          )}
        </div>
        <div className="px-4 py-2 border-t border-border text-xs text-text-subtle flex items-center justify-between">
          <span>按 ESC 关闭</span>
          <span>Ctrl+K 再次打开</span>
        </div>
      </div>
    </div>
  )
}

function App() {
  const [activeTab, setActiveTab] = useState('companion')
  const [showSettings, setShowSettings] = useState(false)
  const [showSearch, setShowSearch] = useState(false)
  const [onboardingComplete, setOnboardingComplete] = useState(null)
  const [checkingOnboarding, setCheckingOnboarding] = useState(true)
  // 【黑屏修复】初始用 light 主题，避免 dark 主题（--bg: 9 9 17 几乎纯黑）
  // 启动后看起来"一片黑"。BackendBooting 已经是 hardcoded 浅色，但 body
  // 还是被 index.css 的 background: rgb(var(--bg)) 覆盖。先用 light 让
  // 整体也保持浅色，等用户进入设置后再切换 dark。
  const [theme, setTheme] = useState('light')
  // 【黑屏修复】Go 后端就绪状态：未就绪时显示 BackendBooting 提示，
  // 而不是渲染可能依赖 Go 方法的子组件。
  // 不再仅依赖 window.go 是否被注入，而是轮询后端 IsReady() 直到 true。
  const [backendReady, setBackendReady] = useState(false)
  // 启动阶段文案（来自后端 GetInitPhase().label）
  const [initPhaseLabel, setInitPhaseLabel] = useState('正在准备资源…')

  // 启动时加载主题（不依赖后端就绪，本地默认即可）
  useEffect(() => {
    applyTheme('light')
  }, [])

  // 【黑屏修复核心】轮询后端 IsReady() 状态。
  // 1) 必须有一个上限（最多 90 秒），避免无限等待
  // 2) 一旦后端报告就绪，立刻设置 backendReady 触发主界面渲染
  // 3) 期间通过 GetInitPhase() 持续拉取阶段文案，做到"Along 正在启动 - 加载服务模块…"
  // 4) 同时监听后端主动推送的 "backend:phase" / "backend:ready" 事件加速收敛
  useEffect(() => {
    let attempts = 0
    const maxAttempts = 900 // 90 秒（每 100ms 一次）
    let timer = null
    let cancelled = false
    let firstPollAt = 0

    const poll = async () => {
      if (cancelled) return
      // 先确认 window.go 已注入（前端桥接完成）
      if (!hasBackend()) {
        if (attempts === 0) {
          console.log('[boot] waiting for window.go injection…')
        }
        if (attempts < maxAttempts) {
          attempts++
          timer = setTimeout(poll, 100)
        } else {
          // hasBackend 一直为 false，强制进入 UI（即便有功能故障也至少能进）
          console.warn('[boot] window.go 注入超时（90s），强制进入主界面')
          setBackendReady(true)
        }
        return
      }
      if (firstPollAt === 0) {
        firstPollAt = Date.now()
        console.log('[boot] window.go 已就绪，开始轮询后端 IsReady()')
      }
      // 拉取阶段文案 + ready
      try {
        if (window.go?.main?.App?.GetInitPhase) {
          const phase = await window.go.main.App.GetInitPhase()
          if (phase && phase.label) {
            setInitPhaseLabel(phase.label)
          }
          if (phase && phase.ready) {
            console.log('[boot] 后端已就绪，总耗时', Date.now() - firstPollAt, 'ms')
            setBackendReady(true)
            return
          }
        } else {
          console.warn('[boot] window.go.main.App.GetInitPhase 不存在')
        }
      } catch (e) {
        // GetInitPhase 可能在极端情况下抛错，吞掉继续轮询
        if (attempts % 20 === 0) {
          console.warn('[boot] GetInitPhase 调用失败:', e?.message || e)
        }
      }
      if (attempts < maxAttempts) {
        attempts++
        timer = setTimeout(poll, 150)
      } else {
        // 超时也不再卡死，让 UI 至少能进
        console.warn('[boot] 轮询 90s 后端仍未就绪，强制进入主界面')
        setBackendReady(true)
      }
    }
    poll()
    return () => {
      cancelled = true
      if (timer) clearTimeout(timer)
    }
  }, [])

  // 监听后端主动推送的阶段事件（asyncInit 完成时会推一次）
  useEffect(() => {
    if (!window.runtime || !window.runtime.EventsOn) return
    const onPhase = (data) => {
      if (!data) return
      if (data.label) setInitPhaseLabel(data.label)
      if (data.ready) setBackendReady(true)
    }
    const onReady = () => setBackendReady(true)
    try {
      window.runtime.EventsOn('backend:phase', onPhase)
      window.runtime.EventsOn('backend:ready', onReady)
    } catch (e) {}
    return () => {
      try {
        if (window.runtime && window.runtime.EventsOff) {
          window.runtime.EventsOff('backend:phase')
          window.runtime.EventsOff('backend:ready')
        }
      } catch (e) {}
    }
  }, [])

  // 检查引导状态（仅后端就绪后才发起 Go 方法调用）
  useEffect(() => {
    if (!backendReady) return
    const checkOnboarding = async () => {
      try {
        const result = await window.go.main.App.IsOnboardingComplete()
        setOnboardingComplete(result)
      } catch (err) {
        console.error('检查引导状态失败:', err)
        setOnboardingComplete(true)
      } finally {
        setCheckingOnboarding(false)
      }
    }
    checkOnboarding()
  }, [backendReady])

  // 全局快捷键
  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setShowSearch(true)
      }
      if ((e.ctrlKey || e.metaKey) && e.key === ',') {
        e.preventDefault()
        setShowSettings(true)
      }
      if (e.key === 'Escape') {
        setShowSearch(false)
        setShowSettings(false)
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // 监听系统托盘导航事件
  useEffect(() => {
    const handleNavigate = (page) => {
      if (page === 'settings') {
        setShowSettings(true)
      } else if (page === 'search') {
        setShowSearch(true)
      } else {
        setActiveTab(page)
      }
    }
    if (window.runtime) {
      window.runtime.EventsOn('navigate', handleNavigate)
    }
    return () => {
      if (window.runtime) {
        window.runtime.EventsOff('navigate')
      }
    }
  }, [])

  const handleOnboardingComplete = () => setOnboardingComplete(true)

  const handleSettingsClose = useCallback((newSettings) => {
    setShowSettings(false)
    if (newSettings && newSettings.theme) {
      setTheme(newSettings.theme)
      applyTheme(newSettings.theme)
    }
  }, [])

  const renderContent = () => {
    // 【黑屏修复】Go 后端未就绪时不渲染子页面，避免子页面在
    // await window.go.main.App.XXX() 上卡住。
    if (!backendReady) {
      return <BackendBooting phaseLabel={initPhaseLabel} />
    }
    const pageMap = {
      companion: CompanionPage,
      plan: PlanPage,
      automation: AutomationPage,
      us: UsPage,
      memory: MemoryPage,
    }
    const Page = pageMap[activeTab] || CompanionPage
    return (
      <ErrorBoundary key={activeTab}>
        <Suspense fallback={<PageFallback />}>
          <Page />
        </Suspense>
      </ErrorBoundary>
    )
  }

  // 【黑屏修复】未就绪时直接显示启动提示，不再等 onboarding 检查
  if (!backendReady) {
    return <BackendBooting phaseLabel={initPhaseLabel} />
  }

  if (checkingOnboarding) {
    return (
      <div className="h-screen w-screen flex items-center justify-center bg-bg">
        <LoadingSpinner text="加载中..." />
      </div>
    )
  }

  if (onboardingComplete === false) {
    return (
      <Suspense fallback={<PageFallback />}>
        <OnboardingPage onComplete={handleOnboardingComplete} />
      </Suspense>
    )
  }

  return (
    <div className="h-screen w-screen flex flex-col bg-bg text-text">
      {/* 顶部标题栏 */}
      <header className="h-12 flex items-center justify-between px-4 border-b border-border bg-surface/50 backdrop-blur-sm">
        <div className="flex items-center gap-2">
          <img
            src="/logo.png"
            alt="Along"
            className="w-7 h-7 rounded-full object-cover"
            onError={(e) => { e.target.style.display = 'none' }}
          />
          <h1 className="font-semibold text-sm tracking-wide">Along</h1>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setShowSearch(true)}
            className="p-2 rounded-lg hover:bg-surface-subtle transition-colors text-text-muted hover:text-text"
            title="搜索 (Ctrl+K)"
          >
            <Search className="w-4 h-4" />
          </button>
          <button
            onClick={() => setShowSettings(true)}
            className="p-2 rounded-lg hover:bg-surface-subtle transition-colors text-text-muted hover:text-text"
            title="设置 (Ctrl+,)"
          >
            <Settings className="w-4 h-4" />
          </button>
        </div>
      </header>

      {/* 主内容区 */}
      <main className="flex-1 overflow-hidden relative">
        <div className="h-full overflow-y-auto animate-fade-in">
          {renderContent()}
        </div>
      </main>

      {/* 底部 Tab 导航 */}
      <nav className="h-16 border-t border-border bg-surface/80 backdrop-blur-sm">
        <div className="flex items-center justify-around h-full">
          {tabs.map((tab) => {
            const Icon = tab.icon
            const isActive = activeTab === tab.id
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex flex-col items-center gap-1 px-4 py-1 rounded-xl transition-all duration-200 ${
                  isActive
                    ? 'text-primary-400'
                    : 'text-text-subtle hover:text-text-muted'
                }`}
              >
                <Icon className="w-5 h-5" />
                <span className="text-xs">{tab.label}</span>
              </button>
            )
          })}
        </div>
      </nav>

      {/* 搜索弹窗 */}
      {showSearch && <SearchModal onClose={() => setShowSearch(false)} />}

      {/* 设置弹窗 */}
      {showSettings && (
        <Suspense fallback={null}>
          <SettingsPage onClose={() => handleSettingsClose()} />
        </Suspense>
      )}
    </div>
  )
}

export default App
