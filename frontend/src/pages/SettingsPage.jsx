import React, { useState, useEffect, useCallback } from 'react'
import { X, Key, Monitor, Database, Shield, Moon, Sun, MonitorDot } from 'lucide-react'

function SettingsPage({ onClose }) {
  const [activeTab, setActiveTab] = useState('api')
  const [apiKey, setApiKey] = useState('')
  const [apiProvider, setApiProvider] = useState('deepseek')
  const [theme, setTheme] = useState('dark')
  const [autoStart, setAutoStart] = useState(false)
  const [systemTrayEnabled, setSystemTrayEnabled] = useState(true)
  const [closeBehavior, setCloseBehavior] = useState('tray')
  const [fontSize, setFontSize] = useState('14px')

  // 状态提示：{ type: 'success' | 'error', msg: string, tab: string }
  const [statusMsg, setStatusMsg] = useState(null)
  // 二次确认删除数据
  const [confirmDelete, setConfirmDelete] = useState(false)
  // 加载状态
  const [loading, setLoading] = useState(true)
  // 后端 API 是否可用
  const [apiAvailable, setApiAvailable] = useState(true)

  const tabs = [
    { id: 'api', label: 'API 配置', icon: Key },
    { id: 'system', label: '系统设置', icon: Monitor },
    { id: 'data', label: '数据管理', icon: Database },
    { id: 'privacy', label: '隐私', icon: Shield },
  ]

  // 显示临时状态提示（3 秒后自动清除）
  const showStatus = useCallback((type, msg, tab = null) => {
    setStatusMsg({ type, msg, tab })
    setTimeout(() => setStatusMsg(null), 3000)
  }, [])

  // 获取后端 App 对象
  const getApp = useCallback(() => {
    if (typeof window === 'undefined') return null
    if (!window.go || !window.go.main || !window.go.main || !window.go.main.App) {
      return null
    }
    return window.go.main.App
  }, [])

  // 组件加载时从后端加载所有设置
  useEffect(() => {
    const app = getApp()
    if (!app) {
      setApiAvailable(false)
      setLoading(false)
      return
    }

    let cancelled = false
    const loadSettings = async () => {
      try {
        const settings = await app.GetSettings()
        if (cancelled) return
        if (settings && typeof settings === 'object') {
          if (typeof settings.api_key === 'string') setApiKey(settings.api_key)
          if (typeof settings.api_provider === 'string' && settings.api_provider) {
            setApiProvider(settings.api_provider)
          }
          if (typeof settings.theme === 'string' && settings.theme) {
            setTheme(settings.theme)
          }
          if (typeof settings.auto_start === 'boolean') {
            setAutoStart(settings.auto_start)
          } else if (typeof settings.auto_start === 'string') {
            setAutoStart(settings.auto_start === 'true' || settings.auto_start === '1')
          }
          if (typeof settings.system_tray_enabled === 'boolean') {
            setSystemTrayEnabled(settings.system_tray_enabled)
          } else if (typeof settings.system_tray_enabled === 'string') {
            setSystemTrayEnabled(settings.system_tray_enabled !== 'false' && settings.system_tray_enabled !== '0')
          } else {
            setSystemTrayEnabled(true)
          }
          if (typeof settings.close_behavior === 'string' && settings.close_behavior) {
            setCloseBehavior(settings.close_behavior)
          }
          if (typeof settings.font_size === 'string' && settings.font_size) {
            setFontSize(settings.font_size)
            document.documentElement.style.fontSize = settings.font_size
          }
        }
      } catch (err) {
        console.error('加载设置失败:', err)
        showStatus('error', '加载设置失败: ' + (err?.message || String(err)))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    loadSettings()
    return () => {
      cancelled = true
    }
  }, [getApp, showStatus])

  // 保存单个设置项
  const saveSetting = async (key, value) => {
    const app = getApp()
    if (!app) {
      setApiAvailable(false)
      showStatus('error', '后端 API 不可用')
      return false
    }
    try {
      const stringValue = typeof value === 'boolean' ? String(value) : value
      await app.SaveSetting(key, stringValue)
      return true
    } catch (err) {
      console.error(`保存 ${key} 失败:`, err)
      showStatus('error', `保存失败: ${err?.message || String(err)}`)
      return false
    }
  }

  // 保存 API 配置（Key 和 Provider 一起保存）
  const handleSaveApiKey = async () => {
    const okProvider = await saveSetting('api_provider', apiProvider)
    const okKey = await saveSetting('api_key', apiKey)
    if (okProvider && okKey) {
      showStatus('success', 'API 配置已保存', 'api')
    }
  }

  // 切换 API 提供商时立即保存
  const handleProviderChange = async (e) => {
    const value = e.target.value
    setApiProvider(value)
    const app = getApp()
    if (!app) {
      setApiAvailable(false)
      return
    }
    await saveSetting('api_provider', value)
    showStatus('success', 'API 提供商已保存', 'api')
  }

  // 切换主题时立即应用并保存
  const handleThemeChange = async (value) => {
    setTheme(value)
    // 立即应用主题，避免重启应用
    const root = document.documentElement
    root.classList.remove('light', 'dark')
    let effective = value
    if (value === 'system' || !value) {
      effective = window.matchMedia('(prefers-color-scheme: dark)').matches
        ? 'dark'
        : 'light'
    }
    root.classList.add(effective)
    root.setAttribute('data-theme', effective)

    const ok = await saveSetting('theme', value)
    if (ok) {
      showStatus('success', '主题已保存', 'system')
    }
  }

  // 切换开机启动时立即保存
  const handleAutoStartChange = async () => {
    const next = !autoStart
    setAutoStart(next)
    const ok = await saveSetting('auto_start', next)
    if (ok) {
      showStatus('success', next ? '已开启开机启动' : '已关闭开机启动', 'system')
    }
  }

  // 切换系统托盘时立即保存
  const handleSystemTrayChange = async () => {
    const next = !systemTrayEnabled
    setSystemTrayEnabled(next)
    const ok = await saveSetting('system_tray_enabled', next)
    if (ok) {
      showStatus('success', next ? '已开启系统托盘' : '已关闭系统托盘，关闭窗口将直接退出', 'system')
    }
  }

  // 导出数据
  const handleExportData = async () => {
    const app = getApp()
    if (!app) {
      setApiAvailable(false)
      showStatus('error', '后端 API 不可用', 'data')
      return
    }
    try {
      const filePath = await app.ExportData()
      if (filePath) {
        showStatus('success', `数据已导出到: ${filePath}`, 'data')
      } else {
        showStatus('success', '数据已导出', 'data')
      }
    } catch (err) {
      console.error('导出数据失败:', err)
      showStatus('error', `导出失败: ${err?.message || String(err)}`, 'data')
    }
  }

  // 删除所有数据（二次确认）
  const handleDeleteData = async () => {
    // 第一次点击：进入二次确认状态
    if (!confirmDelete) {
      setConfirmDelete(true)
      return
    }
    const app = getApp()
    if (!app) {
      setApiAvailable(false)
      setConfirmDelete(false)
      showStatus('error', '后端 API 不可用', 'data')
      return
    }
    try {
      await app.DeleteAllData()
      setConfirmDelete(false)
      showStatus('success', '所有数据已删除', 'data')
      // 清空本地状态
      setApiKey('')
    } catch (err) {
      console.error('删除数据失败:', err)
      setConfirmDelete(false)
      showStatus('error', `删除失败: ${err?.message || String(err)}`, 'data')
    }
  }

  // 渲染状态提示横幅
  const renderStatusBanner = (tabId) => {
    if (!statusMsg) return null
    if (statusMsg.tab && statusMsg.tab !== tabId) return null
    const colorClass =
      statusMsg.type === 'success'
        ? 'bg-green-900/30 border-green-700/50 text-green-400'
        : 'bg-red-900/30 border-red-700/50 text-red-400'
    return (
      <div className={`mb-3 px-3 py-2 rounded-lg border text-xs ${colorClass}`}>
        {statusMsg.msg}
      </div>
    )
  }

  // 后端不可用提示
  const renderApiUnavailable = () => (
    <div className="mb-3 px-3 py-2 rounded-lg border border-yellow-700/50 bg-yellow-900/30 text-yellow-400 text-xs">
      后端 API 不可用（window.go 未注入），请确认应用已正确启动。
    </div>
  )

  return (
    <div className="fixed inset-0 z-50 flex justify-end bg-black/40 backdrop-blur-sm animate-fade-in">
      <div className="w-full max-w-md bg-surface border-l border-border h-full flex flex-col animate-slide-in-right">
        {/* 头部 */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-border">
          <h2 className="text-lg font-semibold text-text">设置</h2>
          <button
            onClick={onClose}
            className="p-2 rounded-lg hover:bg-surface-subtle text-text-subtle hover:text-text transition-colors"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* 侧边标签 */}
        <div className="flex overflow-x-auto border-b border-border px-2">
          {tabs.map((tab) => {
            const Icon = tab.icon
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-2 px-4 py-3 text-sm whitespace-nowrap border-b-2 transition-colors ${
                  activeTab === tab.id
                    ? 'border-primary-500 text-primary-400'
                    : 'border-transparent text-text-subtle hover:text-text-muted'
                }`}
              >
                <Icon className="w-4 h-4" />
                {tab.label}
              </button>
            )
          })}
        </div>

        {/* 内容区 */}
        <div className="flex-1 overflow-y-auto p-5">
          {loading && (
            <div className="text-center text-sm text-text-subtle py-8">加载设置中…</div>
          )}

          {/* API 配置 */}
          {!loading && activeTab === 'api' && (
            <div className="space-y-5 animate-fade-in">
              {!apiAvailable && renderApiUnavailable()}
              {renderStatusBanner('api')}
              <div>
                <label className="block text-sm font-medium text-text-muted mb-2">
                  API 提供商
                </label>
                <select
                  value={apiProvider}
                  onChange={handleProviderChange}
                  className="w-full bg-surface-subtle border border-border rounded-lg px-3 py-2.5 text-sm text-text focus:outline-none focus:border-primary-500/50"
                >
                  <option value="deepseek">DeepSeek (DeepSeek Chat)</option>
                  <option value="zhipu">智谱 AI (GLM-4-Flash)</option>
                  <option value="qwen">通义千问 (Qwen-Turbo)</option>
                </select>
              </div>
              <div>
                <label className="block text-sm font-medium text-text-muted mb-2">
                  API Key
                </label>
                <input
                  type="password"
                  value={apiKey}
                  onChange={(e) => setApiKey(e.target.value)}
                  placeholder="输入你的 API Key"
                  className="w-full bg-surface-subtle border border-border rounded-lg px-3 py-2.5 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50"
                />
                <p className="text-xs text-text-subtle mt-1.5">
                  API Key 将加密存储在本地，不会上传到任何服务器。
                </p>
              </div>
              <button
                onClick={handleSaveApiKey}
                disabled={!apiAvailable}
                className="w-full py-2.5 bg-primary-600 hover:bg-primary-500 disabled:opacity-50 disabled:cursor-not-allowed text-white rounded-lg text-sm font-medium transition-colors"
              >
                保存配置
              </button>
            </div>
          )}

          {/* 系统设置 */}
          {!loading && activeTab === 'system' && (
            <div className="space-y-5 animate-fade-in">
              {!apiAvailable && renderApiUnavailable()}
              {renderStatusBanner('system')}
              <div>
                <label className="block text-sm font-medium text-text-muted mb-3">
                  主题
                </label>
                <div className="grid grid-cols-3 gap-2">
                  {[
                    { id: 'dark', label: '深色', icon: Moon },
                    { id: 'light', label: '浅色', icon: Sun },
                    { id: 'system', label: '跟随系统', icon: MonitorDot },
                  ].map((t) => {
                    const Icon = t.icon
                    return (
                      <button
                        key={t.id}
                        onClick={() => handleThemeChange(t.id)}
                        disabled={!apiAvailable}
                        className={`flex flex-col items-center gap-2 p-3 rounded-xl border transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
                          theme === t.id
                            ? 'border-primary-500 bg-primary-500/10 text-primary-400'
                            : 'border-border text-text-subtle hover:border-border-strong hover:text-text-muted'
                        }`}
                      >
                        <Icon className="w-5 h-5" />
                        <span className="text-xs">{t.label}</span>
                      </button>
                    )
                  })}
                </div>
              </div>
              {/* 字体大小 */}
              <div className="flex items-center justify-between py-3">
                <div>
                  <div className="text-sm text-text">字体大小</div>
                  <div className="text-xs text-text-subtle mt-0.5">调整应用界面的字体大小</div>
                </div>
                <select
                  value={fontSize}
                  onChange={(e) => {
                    setFontSize(e.target.value)
                    document.documentElement.style.fontSize = e.target.value
                    window.go?.main?.App?.SaveSetting('font_size', e.target.value)
                  }}
                  className="bg-surface border border-border rounded-lg px-3 py-1.5 text-sm text-text focus:outline-none focus:border-primary-400"
                >
                  <option value="13px">小</option>
                  <option value="14px">默认</option>
                  <option value="15px">中</option>
                  <option value="16px">大</option>
                  <option value="18px">特大</option>
                </select>
              </div>
              <div className="flex items-center justify-between py-3 border-t border-border">
                <div>
                  <div className="text-sm font-medium text-text-muted">系统托盘</div>
                  <div className="text-xs text-text-subtle mt-0.5">启用后关闭窗口将隐藏到托盘</div>
                </div>
                <button
                  onClick={handleSystemTrayChange}
                  disabled={!apiAvailable}
                  className={`w-11 h-6 rounded-full transition-colors relative disabled:opacity-50 disabled:cursor-not-allowed ${
                    systemTrayEnabled ? 'bg-primary-600' : 'bg-bg-hover'
                  }`}
                >
                  <div
                    className={`w-5 h-5 bg-white rounded-full absolute top-0.5 transition-transform ${
                      systemTrayEnabled ? 'translate-x-5' : 'translate-x-0.5'
                    }`}
                  />
                </button>
              </div>
              <div className="flex items-center justify-between py-3 border-t border-border">
                <div>
                  <div className="text-sm font-medium text-text-muted">开机启动</div>
                  <div className="text-xs text-text-subtle mt-0.5">系统启动时自动运行</div>
                </div>
                <button
                  onClick={handleAutoStartChange}
                  disabled={!apiAvailable}
                  className={`w-11 h-6 rounded-full transition-colors relative disabled:opacity-50 disabled:cursor-not-allowed ${
                    autoStart ? 'bg-primary-600' : 'bg-bg-hover'
                  }`}
                >
                  <div
                    className={`w-5 h-5 bg-white rounded-full absolute top-0.5 transition-transform ${
                      autoStart ? 'translate-x-5' : 'translate-x-0.5'
                    }`}
                  />
                </button>
              </div>
              <div className="flex items-center justify-between py-3 border-t border-border">
                <div>
                  <div className="text-sm font-medium text-text-muted">关闭窗口行为</div>
                  <div className="text-xs text-text-subtle mt-0.5">点击窗口关闭按钮时的行为</div>
                </div>
                <select
                  value={closeBehavior}
                  onChange={(e) => {
                    const value = e.target.value
                    setCloseBehavior(value)
                    saveSetting('close_behavior', value)
                  }}
                  disabled={!apiAvailable}
                  className="bg-surface border border-border rounded-lg px-3 py-1.5 text-sm text-text focus:outline-none focus:border-primary-400 disabled:opacity-50"
                >
                  <option value="tray">最小化到托盘</option>
                  <option value="quit">直接退出程序</option>
                  <option value="confirm">询问我</option>
                </select>
              </div>
            </div>
          )}

          {/* 数据管理 */}
          {!loading && activeTab === 'data' && (
            <div className="space-y-4 animate-fade-in">
              {!apiAvailable && renderApiUnavailable()}
              {renderStatusBanner('data')}
              <div className="bg-surface/60 border border-border rounded-xl p-4">
                <h3 className="text-sm font-medium text-text mb-1">导出数据</h3>
                <p className="text-xs text-text-subtle mb-3">
                  将所有记忆、对话和项目数据导出为 JSON 文件。
                </p>
                <button
                  onClick={handleExportData}
                  disabled={!apiAvailable}
                  className="px-4 py-2 bg-bg-hover hover:bg-border-strong disabled:opacity-50 disabled:cursor-not-allowed text-text rounded-lg text-sm transition-colors"
                >
                  导出数据
                </button>
              </div>
              <div className="bg-surface/60 border border-red-900/30 rounded-xl p-4">
                <h3 className="text-sm font-medium text-red-400 mb-1">删除所有数据</h3>
                <p className="text-xs text-text-subtle mb-3">
                  清除所有本地数据，包括记忆、对话记录和设置。此操作不可恢复。
                </p>
                <button
                  onClick={handleDeleteData}
                  disabled={!apiAvailable}
                  className={`px-4 py-2 rounded-lg text-sm transition-colors disabled:opacity-50 disabled:cursor-not-allowed ${
                    confirmDelete
                      ? 'bg-red-600 hover:bg-red-500 text-white'
                      : 'bg-red-900/30 hover:bg-red-900/50 text-red-400'
                  }`}
                >
                  {confirmDelete ? '再次点击以确认删除（不可恢复）' : '删除所有数据'}
                </button>
                {confirmDelete && (
                  <button
                    onClick={() => setConfirmDelete(false)}
                    className="ml-2 px-4 py-2 bg-bg-hover hover:bg-border-strong text-text-muted rounded-lg text-sm transition-colors"
                  >
                    取消
                  </button>
                )}
              </div>
              {/* 退出应用 */}
              <div className="flex items-center justify-between py-3 border-t border-border mt-2">
                <div>
                  <div className="text-sm text-text">退出应用</div>
                  <div className="text-xs text-text-subtle mt-0.5">完全退出程序（包括系统托盘）</div>
                </div>
                <button
                  onClick={() => {
                    if (window.confirm('确定要退出应用吗？')) {
                      window.go?.main?.App?.QuitApp?.()
                    }
                  }}
                  className="px-4 py-1.5 text-sm bg-danger-500 text-white rounded-lg hover:bg-danger-600 transition-colors"
                >
                  退出
                </button>
              </div>
            </div>
          )}

          {/* 隐私 */}
          {!loading && activeTab === 'privacy' && (
            <div className="space-y-4 animate-fade-in">
              {renderStatusBanner('privacy')}
              <div className="bg-surface/60 border border-border rounded-xl p-4">
                <h3 className="text-sm font-medium text-text mb-2">数据存储</h3>
                <p className="text-sm text-text-subtle leading-relaxed">
                  所有数据仅存储在你的本地设备上，不会上传到任何云端服务器。
                </p>
              </div>
              <div className="bg-surface/60 border border-border rounded-xl p-4">
                <h3 className="text-sm font-medium text-text mb-2">API 安全</h3>
                <p className="text-sm text-text-subtle leading-relaxed">
                  API Key 使用 AES-256 加密存储。所有 API 请求均通过 HTTPS 发送。
                </p>
              </div>
              <div className="bg-surface/60 border border-border rounded-xl p-4">
                <h3 className="text-sm font-medium text-text mb-2">隐私目录</h3>
                <p className="text-sm text-text-subtle leading-relaxed">
                  private/ 目录下的内容不会进入版本控制，确保敏感信息安全。
                </p>
              </div>
            </div>
          )}
        </div>

        {/* 底部版本信息 */}
        <div className="px-5 py-3 border-t border-border text-center">
          <span className="text-xs text-text-subtle">Along v1.0.0</span>
        </div>
      </div>
    </div>
  )
}

export default SettingsPage
