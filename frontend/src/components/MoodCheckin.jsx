import React, { useState, useEffect } from 'react'
import { Smile, Frown, Meh, HeartHandshake, Zap, Moon } from 'lucide-react'

const MOODS = [
  { key: 'happy', label: '开心', icon: Smile, color: 'text-success-500' },
  { key: 'calm', label: '平静', icon: Meh, color: 'text-primary-500' },
  { key: 'sad', label: '低落', icon: Frown, color: 'text-text-subtle' },
  { key: 'excited', label: '兴奋', icon: Zap, color: 'text-warning-500' },
  { key: 'loved', label: '被爱', icon: HeartHandshake, color: 'text-danger-500' },
  { key: 'tired', label: '疲惫', icon: Moon, color: 'text-text-muted' },
]

function MoodCheckin() {
  const [todayMood, setTodayMood] = useState(null)
  const [note, setNote] = useState('')
  const [showNote, setShowNote] = useState(false)
  const [saved, setSaved] = useState(false)

  useEffect(() => {
    loadTodayMood()
  }, [])

  const loadTodayMood = async () => {
    try {
      if (window.go?.main?.App) {
        const result = await window.go.main.App.GetTodayMoodCheckin()
        if (result.checked === 'true') {
          setTodayMood(result.mood)
          if (result.note) setNote(result.note)
          setSaved(true)
        }
      }
    } catch (e) {
      console.error('加载心情打卡失败:', e)
    }
  }

  const handleMoodSelect = async (moodKey) => {
    setTodayMood(moodKey)
    setShowNote(true)
  }

  const handleSave = async () => {
    try {
      if (window.go?.main?.App) {
        await window.go.main.App.SaveMoodCheckin(todayMood, note)
        setSaved(true)
        setShowNote(false)
      }
    } catch (e) {
      console.error('保存心情打卡失败:', e)
    }
  }

  if (saved && todayMood) {
    const mood = MOODS.find(m => m.key === todayMood)
    if (mood) {
      const Icon = mood.icon
      return (
        <div className="flex items-center gap-2 px-3 py-2 bg-surface-subtle rounded-lg border border-border">
          <Icon className={`w-4 h-4 ${mood.color}`} />
          <span className="text-xs text-text-muted">今日心情：{mood.label}</span>
          {note && <span className="text-xs text-text-subtle">· {note}</span>}
        </div>
      )
    }
  }

  return (
    <div className="bg-surface-subtle rounded-lg border border-border p-3">
      <div className="text-xs text-text-muted mb-2">今天心情怎么样？</div>
      <div className="flex gap-2 flex-wrap">
        {MOODS.map(mood => {
          const Icon = mood.icon
          return (
            <button
              key={mood.key}
              onClick={() => handleMoodSelect(mood.key)}
              className={`flex items-center gap-1 px-2 py-1 rounded-md text-xs transition-colors ${
                todayMood === mood.key
                  ? 'bg-primary-500 text-white'
                  : 'bg-surface hover:bg-bg-hover text-text-muted'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {mood.label}
            </button>
          )
        })}
      </div>
      {showNote && (
        <div className="mt-2 flex gap-2">
          <input
            type="text"
            value={note}
            onChange={e => setNote(e.target.value)}
            placeholder="想说点什么吗？（可选）"
            className="flex-1 px-2 py-1 text-xs bg-surface border border-border rounded-md text-text placeholder-text-subtle focus:outline-none focus:border-primary-400"
            maxLength={100}
          />
          <button
            onClick={handleSave}
            className="px-3 py-1 text-xs bg-primary-500 text-white rounded-md hover:bg-primary-600 transition-colors"
          >
            保存
          </button>
        </div>
      )}
    </div>
  )
}

export default MoodCheckin
