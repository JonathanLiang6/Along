import React, { useState, useEffect } from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

function RootApp() {
  const [goReady, setGoReady] = useState(false)

  useEffect(() => {
    let attempts = 0
    const maxAttempts = 50
    const checkGo = () => {
      if (window.go && window.go.main && window.go.main.App) {
        setGoReady(true)
      } else if (attempts < maxAttempts) {
        attempts++
        setTimeout(checkGo, 100)
      } else {
        setGoReady(false)
      }
    }
    checkGo()
  }, [])

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
        <div style={{ textAlign: 'center' }}>
          <div style={{ fontSize: '24px', marginBottom: '12px' }}>正在加载…</div>
          <div style={{ fontSize: '14px', color: '#888' }}>AI Companion</div>
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
