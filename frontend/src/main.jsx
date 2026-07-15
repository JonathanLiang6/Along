import React, { useState, useEffect } from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

function RootApp() {
  const [goReady, setGoReady] = useState(false)
  const [loadFailed, setLoadFailed] = useState(false)
  const [retryKey, setRetryKey] = useState(0)

  useEffect(() => {
    let attempts = 0
    const maxAttempts = 100
    let timer = null
    const checkGo = () => {
      if (window.go && window.go.main && window.go.main.App) {
        setGoReady(true)
        setLoadFailed(false)
      } else if (attempts < maxAttempts) {
        attempts++
        timer = setTimeout(checkGo, 100)
      } else {
        setGoReady(false)
        setLoadFailed(true)
      }
    }
    checkGo()
    return () => {
      if (timer) clearTimeout(timer)
    }
  }, [retryKey])

  const handleRetry = () => {
    setLoadFailed(false)
    setRetryKey(k => k + 1)
  }

  if (!goReady) {
    return (
      <div style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        height: '100vh',
        backgroundColor: '#1c1c1e',
        color: '#fff',
        fontFamily: 'system-ui, sans-serif'
      }}>
        <div style={{ textAlign: 'center', maxWidth: '400px', padding: '20px' }}>
          {loadFailed ? (
            <>
              <div style={{ fontSize: '24px', marginBottom: '12px', color: '#ff453a' }}>加载失败</div>
              <div style={{ fontSize: '14px', color: '#888', marginBottom: '20px' }}>
                后端服务未能正常启动。请尝试重新加载，如果问题持续存在，请检查应用日志。
              </div>
              <button
                onClick={handleRetry}
                style={{
                  padding: '10px 24px',
                  fontSize: '14px',
                  backgroundColor: '#0a84ff',
                  color: '#fff',
                  border: 'none',
                  borderRadius: '8px',
                  cursor: 'pointer'
                }}
              >
                重新加载
              </button>
            </>
          ) : (
            <>
              <div style={{ fontSize: '24px', marginBottom: '12px' }}>正在加载…</div>
              <div style={{ fontSize: '14px', color: '#888' }}>Along</div>
            </>
          )}
        </div>
      </div>
    )
  }

  return (
    <React.StrictMode>
      <App />
    </React.StrictMode>
  )
}

ReactDOM.createRoot(document.getElementById('root')).render(<RootApp />)
