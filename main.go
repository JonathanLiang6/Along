package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// 【黑屏彻底修复】不要再在 wails.Run 之前执行任何 I/O！
	//
	// 历史教训：
	//   旧版 main() 会先调用 NewApp()，里面包含 db.InitDB（创建 14 张表+索引+迁移）、
	//   settings.InitDefaults、initAIClient 等阻塞操作。
	//   这些调用发生在 wails.Run() 之前，意味着：
	//     1) Wails 主循环还没启动 → 窗口还没创建
	//     2) 用户看到的是"程序启动后什么也没发生"的黑屏
	//     3) DB 初始化如果耗时 5~30 秒，黑屏时间就持续 5~30 秒
	//
	// 新方案：
	//   1) 仅创建一个零值 App 结构体（不触发任何 I/O）
	//   2) 立即调用 wails.Run() → Wails 立刻创建窗口并接管主循环
	//   3) 所有重活（DB、Services、Scheduler、Tray）放进 OnStartup 启动的 goroutine
	//   4) 前端 App.jsx 通过 IsReady() / GetInitPhase() 轮询后端就绪状态
	//      期间显示"Along 正在启动"提示
	app := &App{}

	err := wails.Run(&options.App{
		Title:  "Along",
		Width:  1200,
		Height: 800,
		// 初始窗口尺寸由 Wails options 控制；最小尺寸保留以避免用户拖到不可用大小
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		// 使用浅灰作为初始背景色，避免 WebView2 加载首屏前出现黑色闪烁
		BackgroundColour: &options.RGBA{R: 248, G: 250, B: 252, A: 255},
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		// 关闭行为由 beforeClose 完全控制（托盘开启时隐藏，关闭时直接退出）
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
