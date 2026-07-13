import React, { useState, useEffect, useCallback } from 'react'
import { Target, Users, Brain, Settings, Loader2, Search, X, Zap, Bot, User } from 'lucide-react'
import CompanionPage from './pages/CompanionPage'
import PlanPage from './pages/PlanPage'
import UsPage from './pages/UsPage'
import MemoryPage from './pages/MemoryPage'
import SettingsPage from './pages/SettingsPage'
import OnboardingPage from './pages/OnboardingPage'
import AutomationPage from './pages/AutomationPage'

const tabs = [
  { id: 'companion', label: '伙伴', icon: Bot },
  { id: 'plan', label: '计划', icon: Target },
  { id: 'automation', label: '自动化', icon: Zap },
  { id: 'us', label: '我们', icon: Users },
  { id: 'memory', label: '记忆', icon: Brain },
]

// 检查后端是否可用
const hasBackend = () => {
  try {
    return (
      typeof window !== 'undefined' &&
      window.go &&
      window.go.main &&
      window.go.main.App
    )
  } catch (e) {
    return false
  }
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
  const [theme, setTheme] = useState('dark')

  // 启动时加载主题
  useEffect(() => {
    const backend = hasBackend()
    if (backend) {
      window.go.main.App
        .GetSettings()
        .then((s) => {
          if (s && s.theme) {
            setTheme(s.theme)
            applyTheme(s.theme)
          } else {
            applyTheme('dark')
          }
        })
        .catch(() => applyTheme('dark'))
    } else {
      applyTheme('dark')
    }
  }, [])

  // 检查引导状态
  useEffect(() => {
    const backend = hasBackend()
    if (!backend) {
      setOnboardingComplete(true)
      setCheckingOnboarding(false)
      return
    }

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
  }, [])

  // 全局快捷键
  useEffect(() => {
    const onKey = (e) => {
      // Ctrl+K 打开搜索
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setShowSearch(true)
      }
      // Ctrl+, 打开设置
      if ((e.ctrlKey || e.metaKey) && e.key === ',') {
        e.preventDefault()
        setShowSettings(true)
      }
      // ESC 关闭弹窗
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
    // 重新加载主题
    if (newSettings && newSettings.theme) {
      setTheme(newSettings.theme)
      applyTheme(newSettings.theme)
    }
  }, [])

  const renderContent = () => {
    switch (activeTab) {
      case 'companion':
        return <CompanionPage />
      case 'plan':
        return <PlanPage />
      case 'automation':
        return <AutomationPage />
      case 'us':
        return <UsPage />
      case 'memory':
        return <MemoryPage />
      default:
        return <CompanionPage />
    }
  }

  if (checkingOnboarding) {
    return (
      <div className="h-screen w-screen flex items-center justify-center bg-bg">
        <div className="flex items-center gap-2 text-text-muted">
          <Loader2 className="w-5 h-5 animate-spin" />
          <span>加载中...</span>
        </div>
      </div>
    )
  }

  if (onboardingComplete === false) {
    return <OnboardingPage onComplete={handleOnboardingComplete} />
  }

  return (
    <div className="h-screen w-screen flex flex-col bg-bg text-text">
      {/* 顶部标题栏 */}
      <header className="h-12 flex items-center justify-between px-4 border-b border-border bg-surface/50 backdrop-blur-sm">
        <div className="flex items-center gap-2">
          <img
            src="./src/assets/logo.png"
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
        <SettingsPage
          onClose={() => handleSettingsClose()}
        />
      )}
    </div>
  )
}

export default App
