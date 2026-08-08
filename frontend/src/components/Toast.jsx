import React, { useState, useCallback } from 'react'
import { Bell, AlertTriangle, CheckCircle2, X } from 'lucide-react'
import { useEvents } from '../hooks/useEvents'

/**
 * 全局通知 hook — 监听后端 `automation:notification` 事件，
 * 返回一个 push 函数与当前通知列表，供 Toast 容器与页面组件使用。
 */
export function useNotifications() {
  const [items, setItems] = useState([])

  const dismiss = useCallback((id) => {
    setItems((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const push = useCallback((toast) => {
    const id = toast.id || `${Date.now()}_${Math.random().toString(36).slice(2, 8)}`
    setItems((prev) => [
      ...prev.slice(-4), // 最多保留 5 条，防止堆叠过多
      {
        id,
        kind: toast.kind || 'info',
        title: toast.title || '',
        content: toast.content || '',
        ...toast,
      },
    ])
    // 自动消失
    const ttl = typeof toast.ttl === 'number' ? toast.ttl : 6000
    if (ttl > 0) {
      setTimeout(() => {
        setItems((prev) => prev.filter((t) => t.id !== id))
      }, ttl)
    }
  }, [])

  // 监听后端通知事件（经共享事件总线，避免 EventsOff 清掉其他订阅方）
  useEvents('automation:notification', (payload) => {
    if (!payload) return
    push({
      kind: payload.kind || 'info',
      title: payload.title || '通知',
      content: payload.content || '',
    })
  })

  return { items, push, dismiss }
}

const kindStyles = {
  reminder: {
    icon: Bell,
    ring: 'text-primary-400 bg-primary-400/10',
    border: 'border-primary-400/30',
  },
  success: {
    icon: CheckCircle2,
    ring: 'text-emerald-400 bg-emerald-400/10',
    border: 'border-emerald-400/30',
  },
  error: {
    icon: AlertTriangle,
    ring: 'text-danger-400 bg-danger-400/10',
    border: 'border-danger-400/30',
  },
  warning: {
    icon: AlertTriangle,
    ring: 'text-amber-400 bg-amber-400/10',
    border: 'border-amber-400/30',
  },
  info: {
    icon: Bell,
    ring: 'text-text-muted bg-surface-subtle',
    border: 'border-border',
  },
}

/**
 * 全局 Toast 容器 — 右上角堆叠展示通知。
 * 直接在 App 根部渲染一次；内部自行订阅后端通知事件。
 */
export default function ToastContainer() {
  const { items, dismiss } = useNotifications()

  if (items.length === 0) return null

  return (
    <div className="fixed top-14 right-4 z-[100] flex flex-col gap-2 items-end pointer-events-none">
      {items.map((toast) => {
        const style = kindStyles[toast.kind] || kindStyles.info
        const Icon = style.icon
        return (
          <div
            key={toast.id}
            className={`pointer-events-auto w-80 max-w-[calc(100vw-2rem)] bg-surface/95 backdrop-blur border ${style.border} rounded-xl shadow-2xl overflow-hidden animate-toast-in`}
          >
            <div className="flex items-start gap-3 px-3 py-2.5">
              <span className={`shrink-0 mt-0.5 p-1.5 rounded-lg ${style.ring}`}>
                <Icon className="w-4 h-4" />
              </span>
              <div className="flex-1 min-w-0">
                <div className="flex items-center justify-between gap-2">
                  <p className="text-sm font-medium text-text truncate">{toast.title}</p>
                  <button
                    onClick={() => dismiss(toast.id)}
                    className="shrink-0 p-0.5 text-text-subtle hover:text-text rounded transition-colors"
                  >
                    <X className="w-3.5 h-3.5" />
                  </button>
                </div>
                <p className="text-xs text-text-muted mt-0.5 whitespace-pre-wrap break-words max-h-24 overflow-y-auto">
                  {toast.content}
                </p>
              </div>
            </div>
          </div>
        )
      })}
    </div>
  )
}
