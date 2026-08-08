import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './index.css'

// 【黑屏问题修复】不再因为 window.go 没就绪就阻塞渲染。
// 旧逻辑：JS 启动 → 等 window.go.main.App → 10秒后超时才显示错误。
//       一旦 OnStartup 卡住，前端永远停在"正在加载…"，看起来就是黑屏。
// 新逻辑：JS 启动 → 立刻渲染 App → App 内部按需等待 Go 后端。
//       即使 Go 后端长时间未就绪，UI 主框架也会立刻可见，
//       让用户至少看到"Along 启动中"提示。
function RootApp() {
  return (
    <React.StrictMode>
      <App />
    </React.StrictMode>
  )
}

console.log('[boot] main.jsx loaded, mounting React')

ReactDOM.createRoot(document.getElementById('root')).render(<RootApp />)

// 【黑屏修复】React 一旦挂载，立刻摘掉 index.html 里的静态 boot-splash。
// 关键：必须使用 remove() 而不是 display:none，否则 boot-splash
// 仍然以 z-index:9999 浮在所有 React 内容之上、把后端"启动中"提示
// 完全盖住——这就是用户看到的"卡在黑屏"的真正原因之一。
// 这里同步在挂载后立即调用，不依赖任何 setTimeout 兜底。
try {
  const splash = document.getElementById('boot-splash')
  if (splash && splash.parentNode) {
    splash.parentNode.removeChild(splash)
    console.log('[boot] boot-splash removed by React mount')
  }
} catch (e) {
  console.warn('[boot] failed to remove boot-splash:', e)
}

