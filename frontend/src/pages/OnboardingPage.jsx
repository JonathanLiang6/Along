import React, { useState } from 'react'
import { ArrowRight, ArrowLeft, Loader2, Check, User, Key, Sparkles } from 'lucide-react'

const STEPS = {
  WELCOME: 0,
  NAME: 1,
  API_KEY: 2,
  COMPLETE: 3,
}

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

function OnboardingPage({ onComplete }) {
  const [step, setStep] = useState(STEPS.WELCOME)
  const [userName, setUserName] = useState('')
  const [apiKey, setApiKey] = useState('')
  const [apiProvider, setApiProvider] = useState('deepseek')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const handleNext = async () => {
    setError('')

    if (step === STEPS.WELCOME) {
      setStep(STEPS.NAME)
      return
    }

    if (step === STEPS.NAME) {
      if (!userName.trim()) {
        setError('请输入你的名字')
        return
      }
      setStep(STEPS.API_KEY)
      return
    }

    if (step === STEPS.API_KEY) {
      // 保存设置并完成引导
      setLoading(true)
      try {
        const backend = hasBackend()
        if (backend) {
          // 保存 API Provider
          await window.go.main.App.SaveSetting('api_provider', apiProvider)
          // 保存 API Key（如果填写了）
          if (apiKey.trim()) {
            await window.go.main.App.SaveSetting('api_key', apiKey.trim())
          }
          // 完成引导流程
          await window.go.main.App.CompleteOnboarding(userName.trim())
        }
        setStep(STEPS.COMPLETE)
        // 延迟后调用完成回调
        setTimeout(() => {
          if (onComplete) onComplete()
        }, 1500)
      } catch (err) {
        setError(err?.message || String(err) || '保存失败')
      } finally {
        setLoading(false)
      }
      return
    }
  }

  const handleBack = () => {
    setError('')
    if (step > STEPS.WELCOME && step !== STEPS.COMPLETE) {
      setStep(step - 1)
    }
  }

  const handleSkipApiKey = async () => {
    setLoading(true)
    setError('')
    try {
      const backend = hasBackend()
      if (backend) {
        // 即使跳过 API Key，也要保存 provider
        await window.go.main.App.SaveSetting('api_provider', apiProvider)
        await window.go.main.App.CompleteOnboarding(userName.trim())
      }
      setStep(STEPS.COMPLETE)
      setTimeout(() => {
        if (onComplete) onComplete()
      }, 1500)
    } catch (err) {
      setError(err?.message || String(err) || '保存失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="h-screen w-screen flex items-center justify-center bg-bg overflow-hidden">
      {/* 背景装饰 */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute -top-40 -right-40 w-80 h-80 rounded-full bg-primary-600/10 blur-3xl" />
        <div className="absolute -bottom-40 -left-40 w-80 h-80 rounded-full bg-primary-600/5 blur-3xl" />
      </div>

      <div className="relative w-full max-w-md px-6 animate-fade-in">
        {/* 进度指示器 */}
        {step !== STEPS.COMPLETE && (
          <div className="flex items-center justify-center gap-2 mb-6">
            {[STEPS.WELCOME, STEPS.NAME, STEPS.API_KEY].map((s) => (
              <div
                key={s}
                className={`h-1 rounded-full transition-all duration-300 ${
                  s === step
                    ? 'w-8 bg-primary-500'
                    : s < step
                    ? 'w-4 bg-primary-600'
                    : 'w-4 bg-bg-hover'
                }`}
              />
            ))}
          </div>
        )}

        {/* 欢迎页 */}
        {step === STEPS.WELCOME && (
          <div className="text-center">
            <div className="mb-8">
              <div className="w-20 h-20 mx-auto rounded-full bg-gradient-to-br from-primary-500 to-primary-700 flex items-center justify-center mb-6 animate-pulse-slow shadow-lg shadow-primary-600/30">
                <Bot size={40} className="text-white" />
              </div>
              <h1 className="text-2xl font-semibold text-text mb-3">
                你好
              </h1>
              <p className="text-text-muted text-sm leading-relaxed">
                我会陪伴你，
                <br />
                记录你的故事，见证你的成长。
                <br />
                <span className="text-text-subtle text-xs mt-2 block">
                  让我们开始吧。
                </span>
              </p>
            </div>
            <button
              onClick={handleNext}
              className="w-full py-3 bg-primary-600 hover:bg-primary-500 text-white rounded-xl font-medium transition-colors flex items-center justify-center gap-2"
            >
              开始
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        )}

        {/* 输入名字 */}
        {step === STEPS.NAME && (
          <div className="text-center">
            <div className="mb-8">
              <div className="w-16 h-16 mx-auto rounded-full bg-surface-subtle border border-border flex items-center justify-center mb-6">
                <User className="w-8 h-8 text-primary-400" />
              </div>
              <h2 className="text-xl font-semibold text-text mb-3">
                告诉我你的名字
              </h2>
              <p className="text-text-muted text-sm mb-6">
                你可以告诉我你的名字吗？
              </p>
              <input
                type="text"
                value={userName}
                onChange={(e) => {
                  setUserName(e.target.value)
                  setError('')
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') handleNext()
                }}
                placeholder="你的名字"
                maxLength={20}
                autoFocus
                className="w-full bg-surface-subtle border border-border rounded-xl px-4 py-3 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 focus:ring-2 focus:ring-primary-500/20 transition-all"
              />
              {error && (
                <p className="text-red-400 text-xs mt-2">{error}</p>
              )}
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleBack}
                className="px-4 py-3 bg-surface-subtle hover:bg-bg-hover text-text-muted rounded-xl font-medium transition-colors flex items-center justify-center gap-2"
              >
                <ArrowLeft className="w-4 h-4" />
              </button>
              <button
                onClick={handleNext}
                disabled={!userName.trim()}
                className="flex-1 py-3 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white rounded-xl font-medium transition-colors flex items-center justify-center gap-2"
              >
                下一步
                <ArrowRight className="w-4 h-4" />
              </button>
            </div>
          </div>
        )}

        {/* API Key 配置 */}
        {step === STEPS.API_KEY && (
          <div className="text-center">
            <div className="mb-6">
              <div className="w-16 h-16 mx-auto rounded-full bg-surface-subtle border border-border flex items-center justify-center mb-6">
                <Key className="w-8 h-8 text-primary-400" />
              </div>
              <h2 className="text-xl font-semibold text-text mb-3">
                配置 AI 服务
              </h2>
              <p className="text-text-muted text-sm mb-6">
                为了让我更好地帮助你，需要配置一下 AI 服务。
                <br />
                <span className="text-text-subtle text-xs">
                  Key 加密存储在本地，不会上传。
                </span>
              </p>

              <div className="space-y-3 text-left">
                <div>
                  <label className="block text-xs text-text-muted mb-1.5">
                    API 提供商
                  </label>
                  <select
                    value={apiProvider}
                    onChange={(e) => setApiProvider(e.target.value)}
                    className="w-full bg-surface-subtle border border-border rounded-xl px-4 py-2.5 text-sm text-text focus:outline-none focus:border-primary-500/50 focus:ring-2 focus:ring-primary-500/20"
                  >
                    <option value="deepseek">DeepSeek (推荐)</option>
                    <option value="zhipu">智谱 AI (GLM-4-Flash)</option>
                    <option value="qwen">通义千问 (Qwen-Turbo)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs text-text-muted mb-1.5">
                    API Key <span className="text-text-subtle">（可稍后配置）</span>
                  </label>
                  <input
                    type="password"
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder="sk-..."
                    className="w-full bg-surface-subtle border border-border rounded-xl px-4 py-2.5 text-sm text-text placeholder-text-subtle focus:outline-none focus:border-primary-500/50 focus:ring-2 focus:ring-primary-500/20"
                  />
                </div>
              </div>

              {error && (
                <p className="text-red-400 text-xs mt-2 text-left">{error}</p>
              )}
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleBack}
                disabled={loading}
                className="px-4 py-3 bg-surface-subtle hover:bg-bg-hover disabled:opacity-50 text-text-muted rounded-xl font-medium transition-colors flex items-center justify-center"
              >
                <ArrowLeft className="w-4 h-4" />
              </button>
              <button
                onClick={handleSkipApiKey}
                disabled={loading}
                className="flex-1 py-3 bg-surface-subtle hover:bg-bg-hover disabled:opacity-50 text-text-muted rounded-xl font-medium transition-colors"
              >
                稍后配置
              </button>
              <button
                onClick={handleNext}
                disabled={loading}
                className="flex-1 py-3 bg-primary-600 hover:bg-primary-500 disabled:bg-bg-hover disabled:text-text-subtle text-white rounded-xl font-medium transition-colors flex items-center justify-center gap-2"
              >
                {loading ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <>
                    完成
                    <Check className="w-4 h-4" />
                  </>
                )}
              </button>
            </div>
          </div>
        )}

        {/* 完成页 */}
        {step === STEPS.COMPLETE && (
          <div className="text-center">
            <div className="mb-6">
              <div className="w-20 h-20 mx-auto rounded-full bg-gradient-to-br from-green-500 to-green-700 flex items-center justify-center mb-6 animate-fade-in shadow-lg shadow-green-600/30">
                <Check className="w-10 h-10 text-white" />
              </div>
              <h2 className="text-2xl font-semibold text-text mb-3">
                配置完成！
              </h2>
              <p className="text-text-muted text-sm leading-relaxed">
                {userName ? `${userName}，` : ''}现在我们可以开始了。
                <br />
                <span className="inline-flex items-center gap-1 mt-2 text-primary-400">
                  <Sparkles className="w-3 h-3" />
                  今天想聊点什么？
                </span>
              </p>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

export default OnboardingPage
