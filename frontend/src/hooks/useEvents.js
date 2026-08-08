import { useEffect, useRef } from 'react'
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime'

/**
 * 共享事件订阅 hook。
 *
 * Wails 的 EventsOff(eventName) 会移除该事件名下的【全部】监听器，
 * 多个组件直接 EventsOn/EventsOff 同一事件会互相把对方的监听器清掉。
 * 这里为每个事件名只注册一次底层监听器，用 Set 分发到所有订阅方，
 * 仅当最后一个订阅方卸载时才真正 EventsOff。
 */
const registry = new Map() // eventName -> { callbacks: Set<ref>, registered: bool }

function ensureRegistered(eventName) {
  const entry = registry.get(eventName)
  if (!entry || entry.registered) return
  entry.registered = true
  EventsOn(eventName, (payload) => {
    entry.callbacks.forEach((cbRef) => {
      try {
        cbRef.current && cbRef.current(payload)
      } catch (e) {
        console.error(`事件 ${eventName} 处理出错:`, e)
      }
    })
  })
}

function unregisterIfEmpty(eventName) {
  const entry = registry.get(eventName)
  if (entry && entry.callbacks.size === 0 && entry.registered) {
    entry.registered = false
    EventsOff(eventName)
  }
}

export function useEvents(eventName, callback) {
  const cbRef = useRef(callback)
  useEffect(() => {
    cbRef.current = callback
  }, [callback])

  useEffect(() => {
    if (typeof EventsOn !== 'function') return
    if (!registry.has(eventName)) {
      registry.set(eventName, { callbacks: new Set(), registered: false })
    }
    const entry = registry.get(eventName)
    entry.callbacks.add(cbRef)
    ensureRegistered(eventName)
    return () => {
      entry.callbacks.delete(cbRef)
      unregisterIfEmpty(eventName)
    }
  }, [eventName])
}
