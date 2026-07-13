import React, { useState, useEffect } from 'react'
import { MessageCircle, RefreshCw } from 'lucide-react'

function TopicSuggestions({ onSelect }) {
  const [suggestions, setSuggestions] = useState([])
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    loadSuggestions()
  }, [])

  const loadSuggestions = async () => {
    setLoading(true)
    try {
      if (window.go?.main?.App) {
        const result = await window.go.main.App.GetTopicSuggestions()
        if (result && result.length > 0) {
          setSuggestions(result.slice(0, 4))
        }
      }
    } catch (e) {
      console.error('加载话题建议失败:', e)
    } finally {
      setLoading(false)
    }
  }

  if (suggestions.length === 0) return null

  return (
    <div className="mb-3">
      <div className="flex items-center justify-between mb-1.5">
        <span className="text-xs text-text-subtle flex items-center gap-1">
          <MessageCircle className="w-3 h-3" />
          话题建议
        </span>
        <button
          onClick={loadSuggestions}
          className="text-text-subtle hover:text-text-muted transition-colors"
          disabled={loading}
        >
          <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {suggestions.map((s, i) => (
          <button
            key={i}
            onClick={() => onSelect && onSelect(s)}
            className="px-2.5 py-1 text-xs bg-surface-subtle hover:bg-bg-hover text-text-muted rounded-full border border-border transition-colors"
          >
            {s}
          </button>
        ))}
      </div>
    </div>
  )
}

export default TopicSuggestions
