package main

import (
	_ "embed"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/energye/systray"
	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed assets/icon.ico
var trayIcon []byte

type trayQuitCh struct {
	ch     chan struct{}
	closed bool
	mu     sync.Mutex
}

var trayQuit = &trayQuitCh{ch: make(chan struct{})}

func (t *trayQuitCh) Signal() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.closed {
		close(t.ch)
		t.closed = true
	}
}

func (t *trayQuitCh) Wait() <-chan struct{} {
	return t.ch
}

func (t *trayQuitCh) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		t.ch = make(chan struct{})
		t.closed = false
	}
}

type trayState struct {
	ctx         interface{}
	unreadCount int
	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

var tray = &trayState{
	stopCh: make(chan struct{}),
}

func StartTray(app *App) {
	tray.mu.Lock()
	if tray.running {
		tray.mu.Unlock()
		fmt.Println("托盘已在运行，跳过重复启动")
		return
	}
	tray.running = true
	tray.ctx = app
	tray.stopCh = make(chan struct{})
	tray.mu.Unlock()

	trayQuit.Reset()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("托盘崩溃恢复:", r)
			}
		}()
		systray.Run(func() { onReady(app) }, onExit)
	}()
}

func StopTray() {
	tray.mu.Lock()
	if !tray.running {
		tray.mu.Unlock()
		return
	}
	tray.running = false
	close(tray.stopCh)
	tray.mu.Unlock()

	tray.wg.Wait()

	systray.Quit()

	time.Sleep(100 * time.Millisecond)
}

func IsTrayRunning() bool {
	tray.mu.Lock()
	defer tray.mu.Unlock()
	return tray.running
}

func onReady(app *App) {
	systray.SetIcon(trayIcon)
	systray.SetTitle("Along")
	systray.SetTooltip("Along")

	mShow := systray.AddMenuItem("显示主窗口", "显示主窗口")
	mQuickSearch := systray.AddMenuItem("快速搜索", "快速打开搜索功能")
	systray.AddSeparator()

	mPlans := systray.AddMenuItem("我的计划", "查看当前计划")
	mAutomation := systray.AddMenuItem("自动化", "管理自动化任务")
	systray.AddSeparator()

	mStatus := systray.AddMenuItem("正在运行", "Along 状态")
	systray.AddSeparator()

	mSettings := systray.AddMenuItem("设置", "打开设置")
	mAbout := systray.AddMenuItem("关于", "关于 Along")
	systray.AddSeparator()

	mQuit := systray.AddMenuItem("退出", "完全退出程序")

	mShow.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.Show(app.ctx)
			wruntime.WindowUnminimise(app.ctx)
			clearUnread()
		}
	})

	mQuickSearch.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "navigate", "search")
			wruntime.Show(app.ctx)
			clearUnread()
		}
	})

	mPlans.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "navigate", "plan")
			wruntime.Show(app.ctx)
			clearUnread()
		}
	})

	mAutomation.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "navigate", "automation")
			wruntime.Show(app.ctx)
			clearUnread()
		}
	})

	mSettings.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "navigate", "settings")
			wruntime.Show(app.ctx)
		}
	})

	mAbout.Click(func() {
		if app != nil && app.ctx != nil {
			wruntime.EventsEmit(app.ctx, "navigate", "about")
			wruntime.Show(app.ctx)
		}
	})

	mQuit.Click(func() {
		trayQuit.Signal()
	})

	tray.wg.Add(1)
	go updateStatus(mStatus)
}

func onExit() {
	tray.mu.Lock()
	tray.running = false
	tray.mu.Unlock()
	fmt.Println("托盘已退出")
}

func WaitForTrayQuit() <-chan struct{} {
	return trayQuit.Wait()
}

func updateStatus(mStatus *systray.MenuItem) {
	defer tray.wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-tray.stopCh:
			return
		case <-ticker.C:
			tray.mu.Lock()
			count := tray.unreadCount
			tray.mu.Unlock()
			now := time.Now().Format("15:04")
			if count > 0 {
				mStatus.SetTitle(fmt.Sprintf("%d 条新消息 - %s", count, now))
			} else {
				mStatus.SetTitle(fmt.Sprintf("正在运行 - %s", now))
			}
		}
	}
}

func clearUnread() {
	tray.mu.Lock()
	tray.unreadCount = 0
	tray.mu.Unlock()
	updateTrayTitle()
}

func IncrementUnread() {
	tray.mu.Lock()
	tray.unreadCount++
	tray.mu.Unlock()
	updateTrayTitle()
}

func updateTrayTitle() {
	if !IsTrayRunning() {
		return
	}
	tray.mu.Lock()
	count := tray.unreadCount
	tray.mu.Unlock()
	if count > 0 {
		systray.SetTitle(fmt.Sprintf("Along (%d)", count))
	} else {
		systray.SetTitle("Along")
	}
}

func NotifyNewMessage(msg string) {
	if !IsTrayRunning() {
		return
	}
	IncrementUnread()
	tray.mu.Lock()
	app, ok := tray.ctx.(*App)
	tray.mu.Unlock()
	if ok && app != nil && app.ctx != nil {
		wruntime.EventsEmit(app.ctx, "new-message", map[string]interface{}{
			"content": msg,
			"unread":  tray.unreadCount,
		})
	}
}

func ForceQuit() {
	fmt.Println("强制退出应用")
	os.Exit(0)
}
