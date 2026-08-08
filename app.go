package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/core"
	"ai-companion/internal/db"
	"ai-companion/internal/models"
	"ai-companion/internal/pipeline"
	"ai-companion/internal/scheduler"
	"ai-companion/internal/services"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	_ "github.com/mattn/go-sqlite3"
)

// initPhase 表示后端初始化阶段。
// 阶段从 0 开始单调递增，前端通过 GetInitPhase() 轮询以更新启动提示文案。
const (
	phaseBooting      int32 = 0 // OnStartup 刚被调用，尚未开始
	phaseDataDir      int32 = 1 // 数据目录
	phaseDatabase     int32 = 2 // 数据库连接 + 建表 + 迁移
	phaseServices     int32 = 3 // 设置/记忆/对话/计划/自动化服务
	phaseAI           int32 = 4 // AI 客户端 + 默认设置
	phaseCompanion    int32 = 5 // CompanionCore + 工具 Agent 白名单
	phaseScheduler    int32 = 6 // 调度器（异步后台）
	phaseTray         int32 = 7 // 系统托盘（异步后台）
	phaseReady        int32 = 9 // 完全就绪
)

// phaseLabel 给前端展示用的中文文案
var phaseLabel = map[int32]string{
	phaseBooting:   "正在启动…",
	phaseDataDir:   "准备数据目录…",
	phaseDatabase:  "初始化数据库…",
	phaseServices:  "加载服务模块…",
	phaseAI:        "配置 AI 客户端…",
	phaseCompanion: "构建智能体…",
	phaseScheduler: "启动任务调度…",
	phaseTray:      "连接系统托盘…",
	phaseReady:     "准备就绪",
}

// App 应用主结构
//
// 【黑屏修复】字段并发读写说明：
//   - 启动阶段（OnDomReady 之前）：所有字段只能由 asyncInit goroutine 写入
//   - 启动完成后：读侧（前端调用）需加锁，写侧（settings hooks 等）需加锁
//   - a.ready / a.phase 是 atomic.Int32 字段，不需要锁
//   - a.ctx 是 atomic.Pointer 风格：仅在 OnStartup 中写一次，此后只读
//   - a.mu 是兜底互斥锁，用于保护"先 nil 检查 → 再使用"的临界区
type App struct {
	ctx context.Context

	// ready / phase 由 atomic 直接管理，零值即可用
	ready atomic.Int32 // 0=未就绪，1=就绪
	phase atomic.Int32 // 当前初始化阶段

	db               *sql.DB
	aiClient         *ai.Client
	companionCore    *core.CompanionCore
	settings         *services.SettingsService
	memory           *services.MemoryService
	conversation     *services.ConversationService
	plan             *services.PlanService
	automationService *services.AutomationService
	scheduler        *scheduler.Scheduler
	dataDir          string

	shutdownOnce   sync.Once
	isShuttingDown bool
	mu             sync.Mutex
}

// IsReady 对外暴露：后端是否已完全就绪
// 前端 App.jsx 轮询该方法，期间显示"Along 正在启动"提示。
// 即便 window.go 已注入，只要后端初始化没完成，IsReady 也会返回 false。
func (a *App) IsReady() bool {
	return a.ready.Load() == 1
}

// GetInitPhase 对外暴露：当前初始化阶段
// 前端可以根据阶段展示更细粒度的提示文案。
// 返回值为 phaseLabel 中定义的常量。
func (a *App) GetInitPhase() map[string]interface{} {
	phase := a.phase.Load()
	return map[string]interface{}{
		"phase": phase,
		"label": phaseLabel[phase],
		"ready": a.IsReady(),
	}
}

// startup 应用启动时调用
//
// 【黑屏问题彻底修复】Wails 文档与社区共识：
//   - Wails 的 OnStartup 必须在最短时间内返回
//   - 一旦 OnStartup 阻塞，Wails 主循环就被卡住，WebView2 无法继续渲染
//   - "打开软件后一直黑屏"几乎都源于 OnStartup / OnDomReady / 启动钩子中
//     存在长时间阻塞的 I/O、死锁、或与前端通信被同步等待
//
// 本函数采取"极简同步段 + 全异步后台段"：
//   同步段（必须在毫秒级返回，不做任何 I/O、拿锁、阻塞调用）：
//     1) 保存 ctx（唯一必须）
//     2) 立即启动 asyncInit goroutine
//     3) return
//
//   异步段（asyncInit，所有重活都在这里执行）：
//     1) 数据目录 → 数据库 → 服务 → AI 客户端 → CompanionCore（带分阶段超时）
//     2) 调度器 → 托盘（独立 goroutine，绝不阻塞启动）
//     3) 标记 ready = 1，并通过 EventsEmit 通知前端
//
// OnStartup 的目标执行时间：< 1ms
func (a *App) startup(ctx context.Context) {
	enter := time.Now()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[startup] PANIC: %v (耗时 %v)\n", r, time.Since(enter))
		}
	}()

	// ============ 极简同步段：仅保存 ctx，立即启动后台初始化 ============
	a.ctx = ctx
	fmt.Printf("[startup] enter (will launch asyncInit and return immediately)\n")

	// 把所有可能阻塞的工作放到独立 goroutine
	go a.asyncInit()

	fmt.Printf("[startup] exit (耗时 %v)\n", time.Since(enter))
}

// asyncInit 后台异步初始化整个后端
//
// 设计原则：
//   - 严格分阶段（phase），每阶段更新 atomic phase，前端可读
//   - 关键阻塞操作（DB、Scheduler、注册表）放进带超时的子 goroutine
//   - 任一阶段 panic 都被 recover 兜住，不影响整体启动
//   - 全部完成后置 ready=1，并通过 backend:ready 事件推前端
//
// 注意：所有"可能阻塞"的代码（DB Ping、Scheduler Start、注册表写入等）
// 都用 done := make(chan struct{}); go func(){...; close(done)}(); select{...}
// 模式包一层，确保即便底层卡死也不会让 asyncInit 永远不返回。
func (a *App) asyncInit() {
	enter := time.Now()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[asyncInit] panic: %v (耗时 %v)\n", r, time.Since(enter))
		}
		// 即便中间任意阶段 panic，也尝试把 ready 置位，让前端至少能进 UI
		// （部分功能会因为底层没初始化而调用失败，由各方法自身的 nil 检查兜底）
		if a.ready.Load() == 0 {
			a.phase.Store(phaseReady)
			a.ready.Store(1)
			if a.ctx != nil {
				wruntime.EventsEmit(a.ctx, "backend:ready", map[string]interface{}{
					"time":  time.Now().Format(time.RFC3339),
					"phase": phaseReady,
					"label": phaseLabel[phaseReady],
				})
			}
		}
	}()

	// ============ 阶段 1：数据目录（带 3 秒超时） ============
	a.phase.Store(phaseDataDir)
	fmt.Println("[asyncInit] 阶段 1/7: 准备数据目录")
	if err := a.runWithTimeout(3*time.Second, "dataDir", func() error {
		dataDir, err := a.getDataDir()
		if err != nil {
			return err
		}
		a.dataDir = dataDir
		a.ensureDirectories()
		return nil
	}); err != nil {
		fmt.Printf("[asyncInit] 数据目录阶段失败（已跳过）: %v\n", err)
	}

	// ============ 阶段 2：数据库（带 10 秒超时） ============
	a.phase.Store(phaseDatabase)
	fmt.Println("[asyncInit] 阶段 2/7: 初始化数据库")
	if a.dataDir != "" {
		dbPath := filepath.Join(a.dataDir, "companion.db")
		if err := a.runWithTimeout(10*time.Second, "database", func() error {
			database, err := db.InitDB(dbPath)
			if err != nil {
				return err
			}
			a.db = database
			return nil
		}); err != nil {
			fmt.Printf("[asyncInit] 数据库初始化失败（已跳过）: %v\n", err)
		}
	}

	// ============ 阶段 3：服务模块（带 3 秒超时） ============
	a.phase.Store(phaseServices)
	fmt.Println("[asyncInit] 阶段 3/7: 加载服务模块")
	if a.db != nil {
		if err := a.runWithTimeout(3*time.Second, "services", func() error {
			a.settings = services.NewSettingsService(a.db)
			if a.settings == nil {
				return fmt.Errorf("设置服务创建失败")
			}
			a.memory = services.NewMemoryService(a.db)
			a.conversation = services.NewConversationService(a.db)
			if a.dataDir != "" {
				a.conversation.SetConversationsDir(filepath.Join(a.dataDir, "conversations"))
			}
			a.plan = services.NewPlanService(a.db)
			a.automationService = services.NewAutomationService(a.db)
			return nil
		}); err != nil {
			fmt.Printf("[asyncInit] 服务模块加载失败（已跳过）: %v\n", err)
		}
	}

	// ============ 阶段 4：AI 客户端 + 默认设置（带 3 秒超时） ============
	a.phase.Store(phaseAI)
	fmt.Println("[asyncInit] 阶段 4/7: 配置 AI 客户端")
	if a.settings != nil {
		if err := a.runWithTimeout(3*time.Second, "ai", func() error {
			// 默认设置幂等写入（不影响现有值）
			if err := a.settings.InitDefaults(); err != nil {
				fmt.Printf("[asyncInit] 默认设置初始化失败: %v\n", err)
			}
			a.initAIClient()
			return nil
		}); err != nil {
			fmt.Printf("[asyncInit] AI 阶段失败（已跳过）: %v\n", err)
		}
	}

	// ============ 阶段 5：CompanionCore（无 I/O，毫秒级） ============
	a.phase.Store(phaseCompanion)
	fmt.Println("[asyncInit] 阶段 5/7: 构建智能体")
	if a.aiClient != nil && a.memory != nil && a.conversation != nil && a.plan != nil {
		a.companionCore = core.NewCompanionCore(a.aiClient, a.memory, a.conversation, a.plan)

		// 设置文件生成 Agent 的输出目录
		if fileAgent := a.companionCore.GetFileGenerationAgent(); fileAgent != nil && a.dataDir != "" {
			fileAgent.SetOutputDir(filepath.Join(a.dataDir, "research_docs"))
		}

		// 注入工具 Agent 的文件操作白名单
		if toolAgent := a.companionCore.GetToolAgent(); toolAgent != nil {
			roots := []string{}
			if a.dataDir != "" {
				roots = append(roots, a.dataDir)
				roots = append(roots, filepath.Join(a.dataDir, "research_docs"))
			}
			if homeDir, err := os.UserHomeDir(); err == nil {
				roots = append(roots, homeDir)
			}
			if cwd, err := os.Getwd(); err == nil {
				roots = append(roots, cwd)
			}
			toolAgent.SetAllowRoots(roots)
		}
	}

	// 调度器 + 回调（无 I/O）
	if a.db != nil && a.companionCore != nil {
		a.scheduler = scheduler.New(a.db, a.dataDir, a.automationService)
		a.scheduler.OnExecuteAgentTask = func(execID int, task *models.AutomationTask) *models.AutomationExecution {
			return a.executeAutomationTask(execID, task)
		}
		a.companionCore.SetAutomationService(a.automationService)
		a.companionCore.SetTaskExecutor(a)
	}

	// 设置变更钩子（仅内存写入）
	a.setupSettingHooks()

	// ============ 阶段 6：调度器（异步后台，带 15 秒兜底） ============
	a.phase.Store(phaseScheduler)
	fmt.Println("[asyncInit] 阶段 6/7: 启动任务调度（异步）")
	if a.scheduler != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[asyncInit] 调度器 panic: %v\n", r)
				}
			}()
			done := make(chan struct{})
			go func() {
				if err := a.scheduler.Start(); err != nil {
					fmt.Println("调度器启动失败:", err)
				}
				close(done)
			}()
			select {
			case <-done:
				log.Println("调度器启动完成")
			case <-time.After(15 * time.Second):
				fmt.Println("[asyncInit] 调度器启动超时（15s），已放弃等待，继续运行")
			}
		}()
	}

	// 同步开机启动设置（异步，带 3 秒超时）
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[asyncInit] 同步开机启动 panic: %v\n", r)
			}
		}()
		done := make(chan struct{})
		go func() {
			a.syncAutoStart()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			fmt.Println("[asyncInit] 同步开机启动超时（3s），跳过")
		}
	}()

	// ============ 阶段 7：系统托盘（异步后台） ============
	a.phase.Store(phaseTray)
	fmt.Println("[asyncInit] 阶段 7/7: 连接系统托盘（异步）")
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("[asyncInit] 托盘启动 panic: %v\n", r)
			}
		}()
		trayEnabled := true
		if a.settings != nil {
			trayVal, _ := a.settings.Get("system_tray_enabled")
			if trayVal == "false" || trayVal == "0" {
				trayEnabled = false
			}
		}
		if trayEnabled {
			StartTray(a)
			// 监听托盘退出信号
			go func() {
				<-WaitForTrayQuit()
				fmt.Println("托盘请求退出，正在关闭应用...")
				a.QuitApp()
			}()
		}
	}()

	// ============ 就绪：标记 ready=1，通知前端 ============
	a.phase.Store(phaseReady)
	a.ready.Store(1)
	fmt.Printf("[asyncInit] 全部阶段完成，已就绪（总耗时 %v）\n", time.Since(enter))

	if a.ctx != nil {
		wruntime.EventsEmit(a.ctx, "backend:ready", map[string]interface{}{
			"time":  time.Now().Format(time.RFC3339),
			"phase": phaseReady,
			"label": phaseLabel[phaseReady],
		})
	}
}

// runWithTimeout 在 timeout 时间内执行 fn，超时则放弃并返回 error。
// 即使 fn panic 也只影响本函数返回，不影响调用方。
// 关键作用：把任意可能阻塞的 I/O 包成一个有上限的执行单元，
// 避免 asyncInit 阶段被卡死，导致 phase 永远停在某个数字、
// 前端 IsReady 永远返回 false 的"假死锁"。
func (a *App) runWithTimeout(timeout time.Duration, label string, fn func() error) error {
	done := make(chan struct{})
	var runErr error
	go func() {
		defer func() {
			if r := recover(); r != nil {
				runErr = fmt.Errorf("%s panic: %v", label, r)
			}
			close(done)
		}()
		runErr = fn()
	}()
	select {
	case <-done:
		return runErr
	case <-time.After(timeout):
		return fmt.Errorf("%s timeout after %v", label, timeout)
	}
}

// initAIClient 初始化 AI 客户端
func (a *App) initAIClient() {
	// 读取 provider 设置（默认 deepseek）
	provider, _ := a.settings.Get("api_provider")
	if provider == "" {
		provider = "deepseek"
	}
	apiKey, _ := a.settings.Get("api_key")
	if apiKey == "" {
		fmt.Println("警告：未配置 API Key，请前往设置页面配置您的 API Key")
	}
	a.aiClient = ai.NewClient(provider, apiKey)
}

// syncAutoStart 同步开机启动设置
func (a *App) syncAutoStart() {
	val, _ := a.settings.Get("auto_start")
	enabled := val == "true" || val == "1"
	if err := SetAutoStart(enabled); err != nil {
		fmt.Println("同步开机启动设置失败:", err)
	}
}

// domReady DOM 加载完成时调用
// 在 OnDomReady 中调用 Runtime 是最安全的：此时 webview 已经渲染完前端，
// 所有 Runtime API（EventsEmit / Show / Hide 等）都可以正常工作。
//
// 【黑屏修复】如果此时后端 asyncInit 还没完成，再补发一次 "backend:ready"，
// 因为前端可能没赶上 asyncInit 阶段完成时的那次事件。
// 实际 frontend 仍然以 IsReady() 轮询为主，事件只是加速收敛。
func (a *App) domReady(ctx context.Context) {
	enter := time.Now()
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("[domReady] PANIC: %v\n", r)
		}
	}()
	fmt.Printf("[domReady] enter (前端 DOM 已就绪, backendReady=%v)\n", a.IsReady())

	// 主动告知前端：后端就绪状态
	defer func() {
		if a.ctx == nil {
			return
		}
		phase := a.phase.Load()
		wruntime.EventsEmit(a.ctx, "backend:phase", map[string]interface{}{
			"time":  time.Now().Format(time.RFC3339),
			"phase": phase,
			"label": phaseLabel[phase],
			"ready": a.IsReady(),
		})
	}()

	fmt.Printf("[domReady] exit (耗时 %v)\n", time.Since(enter))
}

// beforeClose 关闭前钩子：返回 true 阻止关闭，返回 false 允许关闭
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	if a.isShuttingDown {
		a.mu.Unlock()
		return false
	}
	a.mu.Unlock()

	// 关键修复：a.settings 可能在 asyncInit 失败/未到达该阶段时为 nil，
	// 直接 a.settings.Get 会 nil 解引用 panic，导致窗口无法关闭。
	if a.settings == nil {
		return false
	}
	behavior, _ := a.settings.Get("close_behavior")
	switch behavior {
	case "quit":
		return false
	case "confirm":
		if IsTrayRunning() {
			result, err := wruntime.MessageDialog(ctx, wruntime.MessageDialogOptions{
				Type:    wruntime.QuestionDialog,
				Title:   "关闭确认",
				Message: "确定要退出应用吗？还是最小化到托盘？",
				Buttons: []string{"退出应用", "最小化到托盘", "取消"},
			})
			if err == nil && result == "退出应用" {
				return false
			}
			wruntime.Hide(ctx)
			return true
		}
		return false
	case "tray", "":
		// 默认行为即"最小化到托盘"。当 close_behavior 为 "tray"
		// 或未设置时，统一走"托盘可用则隐藏"分支。
		if IsTrayRunning() {
			wruntime.Hide(ctx)
			return true
		}
	}
	return false
}

// QuitApp 完全退出应用（供前端调用）
//
// 【关键修复】字段置 nil 必须持 a.mu，否则与并发的公共方法
// （GetAutomationTasks 读 a.scheduler、GetHighlights 读 a.db、
// SaveSetting 读 a.settings 等）形成数据竞争，且可能 nil 解引用 panic。
// 修复策略：持锁把字段取出并置 nil，再在锁外调用 Close/Stop（避免锁内阻塞）。
func (a *App) QuitApp() {
	a.shutdownOnce.Do(func() {
		fmt.Println("Along 正在退出...")

		a.mu.Lock()
		a.isShuttingDown = true
		sched := a.scheduler
		a.scheduler = nil
		client := a.aiClient
		a.aiClient = nil
		database := a.db
		a.db = nil
		a.mu.Unlock()

		if sched != nil {
			sched.Stop()
		}

		StopTray()

		if client != nil {
			client.Close()
		}

		if database != nil {
			database.Close()
		}

		if a.ctx != nil {
			wruntime.Quit(a.ctx)
		}

		fmt.Println("Along 已退出")
	})
}

// shutdown 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	a.QuitApp()
}

// getDataDir 获取数据存储目录（使用项目根目录的 data 目录）
func (a *App) getDataDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}

	// 数据目录：可执行文件旁的 data 目录
	dir := filepath.Join(filepath.Dir(exe), "data")

	// 检查新目录是否已有数据
	dbPath := filepath.Join(dir, "companion.db")
	if _, err := os.Stat(dbPath); err == nil {
		// 已有数据，直接返回
		return dir, nil
	}

	// 需要迁移数据
	os.MkdirAll(dir, 0755)

	// 按优先级检测旧数据目录
	oldDirs := []string{
		filepath.Join(filepath.Dir(exe), "along-pre", "AICompanion"),       // 可执行文件旁的 along-pre/AICompanion
		filepath.Join(filepath.Dir(exe), "..", "along-pre", "AICompanion"), // 上级目录的 along-pre/AICompanion
	}
	if appData := os.Getenv("APPDATA"); appData != "" {
		oldDirs = append(oldDirs, filepath.Join(appData, "AICompanion")) // %APPDATA%\AICompanion
	}

	for _, oldDir := range oldDirs {
		if _, err := os.Stat(filepath.Join(oldDir, "companion.db")); err == nil {
			fmt.Println("迁移旧数据:", oldDir, "->", dir)
			a.copyDir(oldDir, dir)
			break
		}
	}

	return dir, nil
}

// copyDir 递归复制目录内容
func (a *App) copyDir(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			os.MkdirAll(dstPath, 0755)
			a.copyDir(srcPath, dstPath)
		} else {
			data, err := os.ReadFile(srcPath)
			if err == nil {
				os.WriteFile(dstPath, data, 0644)
			}
		}
	}
}

// ensureDirectories 确保必要的目录存在
func (a *App) ensureDirectories() {
	dirs := []string{
		a.dataDir,
		filepath.Join(a.dataDir, "conversations"),
		filepath.Join(a.dataDir, "private"),
		filepath.Join(a.dataDir, "research_docs"),
	}
	for _, dir := range dirs {
		os.MkdirAll(dir, 0755)
	}
}

// ==================== 聊天相关 ====================

// SendMessage 发送消息（前端调用）
func (a *App) SendMessage(content string) (*models.MessageResponse, error) {
	if a.companionCore == nil {
		return nil, fmt.Errorf("应用未初始化")
	}
	reply, emotion, err := a.companionCore.ProcessMessage(content)
	if err != nil {
		return nil, err
	}
	go NotifyNewMessage(reply)
	return &models.MessageResponse{
		Content: reply,
		Emotion: emotion,
	}, nil
}

// SendMessageStream 流式发送消息（实时推送到前端）
// 使用 Wails Events 推送每个片段，前端监听 "chat-stream" 事件
func (a *App) SendMessageStream(content string) error {
	if a.ctx == nil || a.companionCore == nil {
		return fmt.Errorf("应用未初始化")
	}

	go func() {
		fullReply, _, err := a.companionCore.ProcessMessageStream(content, func(chunk ai.StreamChunk) {
			eventData := map[string]interface{}{
				"content":       chunk.Content,
				"done":          chunk.Done,
				"error":         chunk.Error,
				"finish_reason": chunk.FinishReason,
			}
			wruntime.EventsEmit(a.ctx, "chat-stream", eventData)
		})
		if err != nil {
			wruntime.EventsEmit(a.ctx, "chat-stream", map[string]interface{}{
				"error": err.Error(),
				"done":  true,
			})
			return
		}
		// 关键修复：只对最终的完整回复发托盘通知，避免斜杠命令
		// 等异步场景下"占位文案"和"实际结果"导致重复通知。
		if fullReply != "" {
			NotifyNewMessage(fullReply)
		}
	}()

	return nil
}

// SendMessageStreamInConversation 在指定对话中流式发送消息
// 前端监听 "chat-stream" 事件，事件数据包含 conversation_id
func (a *App) SendMessageStreamInConversation(conversationID int, content string) error {
	if a.ctx == nil || a.companionCore == nil {
		return fmt.Errorf("应用未初始化")
	}
	if a.conversation == nil {
		return fmt.Errorf("对话服务未初始化")
	}

	go func() {
		fullReply, _, err := a.companionCore.ProcessMessageStreamInConversation(conversationID, content, func(chunk ai.StreamChunk) {
			eventData := map[string]interface{}{
				"conversation_id": conversationID,
				"content":         chunk.Content,
				"done":            chunk.Done,
				"error":           chunk.Error,
				"finish_reason":   chunk.FinishReason,
			}
			wruntime.EventsEmit(a.ctx, "chat-stream", eventData)
		})
		if err != nil {
			wruntime.EventsEmit(a.ctx, "chat-stream", map[string]interface{}{
				"conversation_id": conversationID,
				"error":           err.Error(),
				"done":            true,
			})
			return
		}
		// 关键修复：只对最终的完整回复发托盘通知，避免斜杠命令
		// 等异步场景下"占位文案"和"实际结果"导致重复通知。
		// 斜杠命令的真实结果已在 companionCore 内部协程完成
		// 持久化，并由该函数返回 fullReply 一次性触发通知。
		if fullReply != "" {
			NotifyNewMessage(fullReply)
		}
	}()

	return nil
}

// ==================== 对话管理 ====================

// CreateConversation 创建新对话
func (a *App) CreateConversation(title string) (*models.Conversation, error) {
	if a.conversation == nil {
		return nil, fmt.Errorf("对话服务未初始化")
	}
	return a.conversation.CreateConversation(title)
}

// ListConversations 获取所有对话列表
func (a *App) ListConversations() ([]models.Conversation, error) {
	if a.conversation == nil {
		return []models.Conversation{}, nil
	}
	return a.conversation.ListConversations()
}

// GetConversation 获取单个对话
func (a *App) GetConversation(id int) (*models.Conversation, error) {
	if a.conversation == nil {
		return nil, fmt.Errorf("对话服务未初始化")
	}
	return a.conversation.GetConversation(id)
}

// RenameConversation 重命名对话
func (a *App) RenameConversation(id int, title string) error {
	if a.conversation == nil {
		return fmt.Errorf("对话服务未初始化")
	}
	return a.conversation.RenameConversation(id, title)
}

// DeleteConversation 删除对话
func (a *App) DeleteConversation(id int) error {
	if a.conversation == nil {
		return fmt.Errorf("对话服务未初始化")
	}
	return a.conversation.DeleteConversation(id)
}

// GetConversationMessages 获取指定对话的消息
func (a *App) GetConversationMessages(conversationID int) ([]models.Message, error) {
	if a.conversation == nil {
		return []models.Message{}, nil
	}
	return a.conversation.GetMessagesByConversationID(conversationID)
}

// GetConversationHistory 获取对话历史（兼容旧接口：按日期）
func (a *App) GetConversationHistory(date string) ([]models.Message, error) {
	if a.conversation == nil {
		return []models.Message{}, nil
	}
	return a.conversation.GetMessages(date)
}

// GetConversationDates 获取有对话的所有日期（用于月度总结与历史选择）
func (a *App) GetConversationDates() ([]string, error) {
	if a.db == nil {
		return []string{}, nil
	}
	rows, err := a.db.Query("SELECT DISTINCT date FROM conversations ORDER BY date DESC LIMIT 365")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var dates []string
	for rows.Next() {
		var d string
		if err := rows.Scan(&d); err != nil {
			continue
		}
		dates = append(dates, d)
	}
	return dates, nil
}

// ==================== 记忆相关 ====================

// GetMemories 获取记忆列表
func (a *App) GetMemories(memoryType string) ([]models.Memory, error) {
	if a.memory == nil {
		return []models.Memory{}, nil
	}
	return a.memory.GetMemories(memoryType)
}

// GetMemoryCountByType 按类型统计记忆数量
func (a *App) GetMemoryCountByType() (map[string]int, error) {
	if a.memory == nil {
		return map[string]int{}, nil
	}
	return a.memory.GetCountByType()
}

// UpdateMemory 更新记忆
func (a *App) UpdateMemory(id int, content string) error {
	if a.memory == nil {
		return fmt.Errorf("记忆服务未初始化")
	}
	return a.memory.UpdateMemory(id, content)
}

// DeleteMemory 删除记忆
func (a *App) DeleteMemory(id int) error {
	if a.memory == nil {
		return fmt.Errorf("记忆服务未初始化")
	}
	return a.memory.DeleteMemory(id)
}

// AddMemory 添加记忆（前端调用）
func (a *App) AddMemory(memoryType, content, source string, confidence float64) error {
	if a.memory == nil {
		return fmt.Errorf("记忆服务未初始化")
	}
	return a.memory.AddMemory(memoryType, content, source, confidence)
}

// ==================== 计划相关 ====================

// GetGoals 获取所有计划
func (a *App) GetGoals() ([]models.Goal, error) {
	if a.plan == nil {
		return []models.Goal{}, nil
	}
	return a.plan.GetAllGoals()
}

// GetGoalsByType 按类型获取计划
func (a *App) GetGoalsByType(goalType string) ([]models.Goal, error) {
	if a.plan == nil {
		return []models.Goal{}, nil
	}
	return a.plan.GetGoalsByType(goalType)
}

// CreateGoal 创建计划
func (a *App) CreateGoal(title, description, goalType string) (*models.Goal, error) {
	if a.plan == nil {
		return nil, fmt.Errorf("计划服务未初始化")
	}
	return a.plan.CreateGoal(title, description, goalType)
}

// UpdateGoal 更新计划
func (a *App) UpdateGoal(id int, title, description, status, currentFocus, nextStep, mood string, progress int) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.UpdateGoal(id, title, description, status, currentFocus, nextStep, mood, progress)
}

// DeleteGoal 删除计划
func (a *App) DeleteGoal(id int) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.DeleteGoal(id)
}

// GetMilestones 获取计划的里程碑
func (a *App) GetMilestones(goalID int) ([]models.Milestone, error) {
	if a.plan == nil {
		return []models.Milestone{}, nil
	}
	return a.plan.GetMilestones(goalID)
}

// AddMilestone 添加里程碑
func (a *App) AddMilestone(goalID int, title, description string) (*models.Milestone, error) {
	if a.plan == nil {
		return nil, fmt.Errorf("计划服务未初始化")
	}
	return a.plan.AddMilestone(goalID, title, description)
}

// UpdateMilestone 更新里程碑
func (a *App) UpdateMilestone(id int, title, description, status string) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.UpdateMilestone(id, title, description, status)
}

// CompleteMilestone 完成里程碑
func (a *App) CompleteMilestone(id int, companionComment string) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.CompleteMilestone(id, companionComment)
}

// DeleteMilestone 删除里程碑
func (a *App) DeleteMilestone(id int) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.DeleteMilestone(id)
}

// GetCheckIns 获取计划的记录
func (a *App) GetCheckIns(goalID int) ([]models.CheckIn, error) {
	if a.plan == nil {
		return []models.CheckIn{}, nil
	}
	return a.plan.GetCheckIns(goalID)
}

// AddCheckIn 添加记录
func (a *App) AddCheckIn(goalID int, content, mood, companionResponse string) (*models.CheckIn, error) {
	if a.plan == nil {
		return nil, fmt.Errorf("计划服务未初始化")
	}
	return a.plan.AddCheckIn(goalID, content, mood, companionResponse)
}

// DeleteCheckIn 删除记录
func (a *App) DeleteCheckIn(id int) error {
	if a.plan == nil {
		return fmt.Errorf("计划服务未初始化")
	}
	return a.plan.DeleteCheckIn(id)
}

// SearchGoals 搜索计划
func (a *App) SearchGoals(keyword string) ([]models.Goal, error) {
	if a.plan == nil {
		return []models.Goal{}, nil
	}
	return a.plan.SearchGoals(keyword)
}

// ==================== 设置相关 ====================

// GetSettings 获取设置
func (a *App) GetSettings() (map[string]string, error) {
	if a.settings == nil {
		return map[string]string{}, nil
	}
	return a.settings.GetAll()
}

// SaveSetting 保存设置
func (a *App) SaveSetting(key, value string) error {
	if a.settings == nil {
		return fmt.Errorf("设置服务未初始化")
	}
	return a.settings.Set(key, value)
}

// setupSettingHooks 注册设置变更钩子（副作用处理）
func (a *App) setupSettingHooks() {
	if a.settings == nil {
		return
	}

	// API Key 变更时更新 AI 客户端
	a.settings.OnChange("api_key", func(key, oldValue, newValue string) error {
		if newValue == "" {
			return nil
		}
		a.mu.Lock()
		client := a.aiClient
		a.mu.Unlock()
		if client != nil {
			client.SetAPIKey(newValue)
		}
		return nil
	})

	// API Provider 变更时重新初始化客户端
	// 关键修复：用 a.mu 保护 a.aiClient 的读写，避免"用户切换 provider
	// 的 goroutine"与"正在发起流式请求的 goroutine"并发读写同一指针
	// 而出现 data race（hook 本身又处于 settings 释放写锁后的
	// 回调上下文，不会死锁）。
	a.settings.OnChange("api_provider", func(key, oldValue, newValue string) error {
		if newValue == "" {
			return nil
		}
		apiKey, _ := a.settings.Get("api_key")

		a.mu.Lock()
		if a.aiClient != nil {
			a.aiClient.SetProvider(newValue)
		} else {
			a.aiClient = ai.NewClient(newValue, apiKey)
		}
		client := a.aiClient
		core := a.companionCore
		a.mu.Unlock()

		if core != nil {
			core.UpdateAIClient(client)
		}
		return nil
	})

	// 开机启动变更
	a.settings.OnChange("auto_start", func(key, oldValue, newValue string) error {
		enabled := newValue == "true" || newValue == "1"
		if err := SetAutoStart(enabled); err != nil {
			fmt.Println("设置开机启动失败:", err)
		}
		return nil
	})

	// 系统托盘变更
	a.settings.OnChange("system_tray_enabled", func(key, oldValue, newValue string) error {
		if newValue == "false" || newValue == "0" {
			StopTray()
		}
		return nil
	})
}

// ==================== 观察与复盘 ====================

// GetObservations 获取观察列表
func (a *App) GetObservations() ([]models.Observation, error) {
	if a.memory == nil {
		return []models.Observation{}, nil
	}
	return a.memory.GetObservations()
}

// GenerateReflection 生成复盘
func (a *App) GenerateReflection(period string) (*models.Reflection, error) {
	if a.companionCore == nil {
		return nil, fmt.Errorf("应用未初始化")
	}
	return a.companionCore.GenerateReflection(period)
}

// ==================== 伙伴状态 ====================

// GetCompanionStatus 获取伙伴状态（动态计算）
func (a *App) GetCompanionStatus() (*models.CompanionStatus, error) {
	status := &models.CompanionStatus{
		Name:       "Along",
		Mood:       "ready",
		LastSeen:   time.Now().Format("2006-01-02 15:04"),
		TrustLevel: 75,
	}

	if a.conversation == nil || a.memory == nil {
		return status, nil
	}

	// 根据最近对话计算情绪
	msgs, err := a.conversation.GetRecentMessages(5)
	if err == nil && len(msgs) > 0 {
		// 分析最近的用户消息情绪
		for _, m := range msgs {
			if m.Role == "user" {
				content := m.Content
				if containsAny(content, []string{"开心", "高兴", "快乐", "哈哈", "棒"}) {
					status.Mood = "开心"
					break
				}
				if containsAny(content, []string{"难过", "伤心", "哭", "失落"}) {
					status.Mood = "关注"
					break
				}
				if containsAny(content, []string{"生气", "愤怒", "烦", "讨厌"}) {
					status.Mood = "支持"
					break
				}
				if containsAny(content, []string{"累", "疲惫", "困", "焦虑"}) {
					status.Mood = "专业"
					break
				}
			}
		}
	}

	// 根据记忆数量和对话天数计算信任度
	memCount, _ := a.memory.GetCountByType()
	totalMem := 0
	for _, c := range memCount {
		totalMem += c
	}

	// 基础信任度50，每条记忆+1（上限90）
	trust := 50 + totalMem
	if trust > 90 {
		trust = 90
	}
	if trust < 50 {
		trust = 50
	}
	status.TrustLevel = trust

	return status, nil
}

// containsAny 检查字符串是否包含任一关键词
func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// ==================== 数据管理 ====================

// ExportData 导出数据（返回 JSON 路径）
func (a *App) ExportData() (string, error) {
	if a.db == nil {
		return "", fmt.Errorf("数据库未初始化")
	}

	exportPath := filepath.Join(a.dataDir, "export.json")

	// 导出所有数据库数据
	var memories []models.Memory
	var goals []models.Goal
	var settings map[string]string
	var observations []models.Observation
	var highlights []models.Highlight
	var reflections []models.Reflection

	if a.memory != nil {
		memories, _ = a.memory.GetMemories("")
		observations, _ = a.memory.GetObservations()
	}
	if a.plan != nil {
		goals, _ = a.plan.GetAllGoals()
	}
	if a.settings != nil {
		settings, _ = a.settings.GetAll()
	}
	highlights, _ = a.GetHighlights()
	reflections, _ = a.GetReflections()

	// 导出所有对话和消息
	conversations, _ := a.getAllConversations()
	messages, _ := a.getAllMessages()

	// 读取 conversations 目录下的 JSON 文件
	conversationFiles := a.exportConversationFiles()

	export := map[string]interface{}{
		"memories":           memories,
		"goals":              goals,
		"conversations":      conversations,
		"messages":           messages,
		"observations":       observations,
		"highlights":         highlights,
		"reflections":        reflections,
		"settings":           settings,
		"conversation_files": conversationFiles,
		"exported_at":        time.Now().Format("2006-01-02 15:04:05"),
	}

	data, err := json.MarshalIndent(export, "", "  ")
	if err != nil {
		return "", err
	}

	err = os.WriteFile(exportPath, data, 0644)
	if err != nil {
		return "", err
	}

	return exportPath, nil
}

// getAllConversations 获取所有对话
func (a *App) getAllConversations() ([]models.Conversation, error) {
	if a.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	rows, err := a.db.Query("SELECT id, date, title, agent_route, created_at FROM conversations ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var conversations []models.Conversation
	for rows.Next() {
		var c models.Conversation
		var title, agentRoute sql.NullString
		if err := rows.Scan(&c.ID, &c.Date, &title, &agentRoute, &c.CreatedAt); err != nil {
			continue
		}
		c.Title = title.String
		c.AgentRoute = agentRoute.String
		conversations = append(conversations, c)
	}
	return conversations, nil
}

// getAllMessages 获取所有消息
func (a *App) getAllMessages() ([]models.Message, error) {
	if a.db == nil {
		return nil, fmt.Errorf("数据库未初始化")
	}
	rows, err := a.db.Query("SELECT id, conversation_id, role, content, emotion, timestamp FROM messages ORDER BY timestamp")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []models.Message
	for rows.Next() {
		var m models.Message
		var emotion sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &emotion, &m.Timestamp); err != nil {
			continue
		}
		if emotion.Valid {
			m.Emotion = emotion.String
		}
		messages = append(messages, m)
	}
	return messages, nil
}

// exportConversationFiles 读取 conversations 目录下的 JSON 文件内容
func (a *App) exportConversationFiles() map[string]string {
	files := make(map[string]string)
	convsDir := filepath.Join(a.dataDir, "conversations")

	entries, err := os.ReadDir(convsDir)
	if err != nil {
		return files
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			filePath := filepath.Join(convsDir, entry.Name())
			data, err := os.ReadFile(filePath)
			if err == nil {
				files[entry.Name()] = string(data)
			}
		}
	}
	return files
}

// DeleteAllData 删除所有数据
func (a *App) DeleteAllData() error {
	if a.db == nil {
		return fmt.Errorf("数据库未初始化")
	}

	// 使用白名单验证表名，防止 SQL 注入
	allowedTables := map[string]bool{
		"memories": true, "conversations": true, "messages": true,
		"goals": true, "milestones": true, "check_ins": true,
		"observations": true, "highlights": true, "reflections": true,
	}

	tables := []string{"memories", "conversations", "messages", "goals", "milestones", "check_ins", "observations", "highlights", "reflections"}
	for _, table := range tables {
		if !allowedTables[table] {
			continue
		}
		a.db.Exec("DELETE FROM " + table)
	}

	// 清理 conversations 目录下的 JSON 文件
	convsDir := filepath.Join(a.dataDir, "conversations")
	entries, err := os.ReadDir(convsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				filePath := filepath.Join(convsDir, entry.Name())
				os.Remove(filePath)
			}
		}
	}

	return nil
}

// ==================== 高光回忆 ====================

// GetHighlights 获取高光回忆
func (a *App) GetHighlights() ([]models.Highlight, error) {
	if a.db == nil {
		return []models.Highlight{}, nil
	}
	rows, err := a.db.Query("SELECT id, title, description, date, memory_ids, user_marked, created_at FROM highlights ORDER BY date DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var highlights []models.Highlight
	for rows.Next() {
		h, err := scanHighlightRows(rows)
		if err != nil {
			continue
		}
		highlights = append(highlights, h)
	}
	return highlights, nil
}

// scanHighlightRows 安全 scan 高光回忆行（处理 NULL 字段）
func scanHighlightRows(rows *sql.Rows) (models.Highlight, error) {
	var h models.Highlight
	var desc, date, memoryIDs sql.NullString
	if err := rows.Scan(&h.ID, &h.Title, &desc, &date, &memoryIDs, &h.UserMarked, &h.CreatedAt); err != nil {
		return h, err
	}
	h.Description = desc.String
	h.Date = date.String
	h.MemoryIDs = memoryIDs.String
	return h, nil
}

// AddHighlight 添加高光回忆
func (a *App) AddHighlight(title, description, date string) error {
	if a.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	_, err := a.db.Exec(
		"INSERT INTO highlights (title, description, date, user_marked, created_at) VALUES (?, ?, ?, 1, datetime('now'))",
		title, description, date,
	)
	return err
}

// DeleteHighlight 删除高光回忆
func (a *App) DeleteHighlight(id int) error {
	if a.db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	_, err := a.db.Exec("DELETE FROM highlights WHERE id = ?", id)
	return err
}

// ==================== 复盘历史 ====================

// GetReflections 获取复盘历史
func (a *App) GetReflections() ([]models.Reflection, error) {
	if a.db == nil {
		return []models.Reflection{}, nil
	}
	rows, err := a.db.Query("SELECT id, period_start, period_end, growth_analysis, relationship_analysis, project_review, observations, created_at FROM reflections ORDER BY created_at DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reflections []models.Reflection
	for rows.Next() {
		var r models.Reflection
		var periodStart, periodEnd, growth, relationship, review, observations sql.NullString
		if err := rows.Scan(&r.ID, &periodStart, &periodEnd, &growth, &relationship, &review, &observations, &r.CreatedAt); err != nil {
			continue
		}
		r.PeriodStart = periodStart.String
		r.PeriodEnd = periodEnd.String
		r.GrowthAnalysis = growth.String
		r.RelationshipAnalysis = relationship.String
		r.ProjectReview = review.String
		r.Observations = observations.String
		reflections = append(reflections, r)
	}
	return reflections, nil
}

// ==================== 工具操作 ====================

// ToolReadFile 读取文件内容（前端直接调用）
func (a *App) ToolReadFile(path string) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.ReadFile(path)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ToolWriteFile 写入文件（前端直接调用）
func (a *App) ToolWriteFile(path, content string) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.WriteFile(path, content)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ToolListDir 列出目录内容（前端直接调用）
func (a *App) ToolListDir(path string) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.ListDir(path)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ToolGitStatus 获取git状态（前端直接调用）
func (a *App) ToolGitStatus(repoPath string) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.GitStatus(repoPath)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ToolGitLog 获取git提交记录（前端直接调用）
func (a *App) ToolGitLog(repoPath string, limit int) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.GitLog(repoPath, limit)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ToolOpenBrowser 打开浏览器链接（前端直接调用）
func (a *App) ToolOpenBrowser(url string) map[string]interface{} {
	if a.companionCore == nil {
		return map[string]interface{}{"success": false, "error": "应用未初始化"}
	}
	toolAgent := a.companionCore.GetToolAgent()
	if toolAgent == nil {
		return map[string]interface{}{
			"success": false,
			"error":   "Tool Agent 未初始化",
		}
	}
	resp := toolAgent.OpenBrowser(url)
	return map[string]interface{}{
		"success": resp.Success,
		"data":    resp.Data,
		"error":   resp.Error,
	}
}

// ==================== 引导流程 ====================

// IsOnboardingComplete 检查是否已完成引导
func (a *App) IsOnboardingComplete() (bool, error) {
	if a.settings == nil {
		return false, fmt.Errorf("设置服务未初始化")
	}
	value, err := a.settings.Get("onboarding_completed")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

// CompleteOnboarding 完成引导流程
func (a *App) CompleteOnboarding(userName string) error {
	if a.settings == nil {
		return fmt.Errorf("设置服务未初始化")
	}
	// 保存用户名字
	if userName != "" {
		if err := a.settings.Set("user_name", userName); err != nil {
			return err
		}
	}
	// 标记引导已完成
	return a.settings.Set("onboarding_completed", "true")
}

// GetUserName 获取用户名字
func (a *App) GetUserName() (string, error) {
	if a.settings == nil {
		return "", nil
	}
	return a.settings.Get("user_name")
}

// ==================== 主动机制 ====================

// GetProactiveContent 获取主动内容
func (a *App) GetProactiveContent() ([]models.Observation, error) {
	if a.companionCore == nil {
		return []models.Observation{}, nil
	}
	return a.companionCore.GenerateProactiveContent()
}

// ==================== 全局搜索 ====================

// GlobalSearch 全局搜索（记忆 + 对话）
func (a *App) GlobalSearch(query string) (map[string]interface{}, error) {
	if query == "" {
		return map[string]interface{}{
			"memories":   []models.Memory{},
			"messages":   []models.Message{},
			"highlights": []models.Highlight{},
		}, nil
	}

	var memories []models.Memory
	if a.memory != nil {
		var mErr error
		memories, mErr = a.memory.SearchMemories(query)
		if mErr != nil {
			memories = []models.Memory{}
		}
	} else {
		memories = []models.Memory{}
	}

	var messages []models.Message
	if a.conversation != nil {
		var cErr error
		messages, cErr = a.conversation.SearchMessages(query, 20)
		if cErr != nil {
			messages = []models.Message{}
		}
	} else {
		messages = []models.Message{}
	}

	highlights, hErr := a.searchHighlights(query)
	if hErr != nil {
		highlights = []models.Highlight{}
	}

	return map[string]interface{}{
		"memories":   memories,
		"messages":   messages,
		"highlights": highlights,
	}, nil
}

func (a *App) searchHighlights(query string) ([]models.Highlight, error) {
	if a.db == nil {
		return []models.Highlight{}, nil
	}
	rows, err := a.db.Query(
		"SELECT id, title, description, date, memory_ids, user_marked, created_at FROM highlights WHERE title LIKE ? OR description LIKE ? ORDER BY date DESC LIMIT 10",
		"%"+query+"%", "%"+query+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var highlights []models.Highlight
	for rows.Next() {
		h, err := scanHighlightRows(rows)
		if err != nil {
			continue
		}
		highlights = append(highlights, h)
	}
	return highlights, nil
}

// ==================== 心情打卡 ====================

// SaveMoodCheckin 保存每日心情打卡
func (a *App) SaveMoodCheckin(mood, note string) error {
	if a.settings == nil {
		return fmt.Errorf("设置服务未初始化")
	}
	today := time.Now().Format("2006-01-02")
	// 使用 settings 表存储，key 格式: mood_checkin_2026-07-10
	key := "mood_checkin_" + today
	value := mood
	if note != "" {
		value = mood + "|" + note
	}
	return a.settings.Set(key, value)
}

// GetTodayMoodCheckin 获取今日心情打卡
func (a *App) GetTodayMoodCheckin() (map[string]string, error) {
	if a.settings == nil {
		return map[string]string{"mood": "", "note": "", "checked": "false"}, nil
	}
	today := time.Now().Format("2006-01-02")
	key := "mood_checkin_" + today
	val, err := a.settings.Get(key)
	if err != nil {
		return nil, err
	}
	if val == "" {
		return map[string]string{"mood": "", "note": "", "checked": "false"}, nil
	}
	parts := strings.SplitN(val, "|", 2)
	result := map[string]string{"mood": parts[0], "checked": "true"}
	if len(parts) > 1 {
		result["note"] = parts[1]
	} else {
		result["note"] = ""
	}
	return result, nil
}

// GetMoodHistory 获取心情打卡历史（最近30天）
func (a *App) GetMoodHistory() ([]map[string]string, error) {
	if a.settings == nil {
		return []map[string]string{}, nil
	}
	all, err := a.settings.GetAll()
	if err != nil {
		return nil, err
	}
	var history []map[string]string
	for k, v := range all {
		if len(k) > 13 && k[:13] == "mood_checkin_" {
			date := k[13:]
			parts := strings.SplitN(v, "|", 2)
			entry := map[string]string{"date": date, "mood": parts[0]}
			if len(parts) > 1 {
				entry["note"] = parts[1]
			}
			history = append(history, entry)
		}
	}
	return history, nil
}

// ==================== 对话话题建议 ====================

// ==================== 自动化任务接口 ====================

// GetAutomationTasks 获取自动化任务列表
func (a *App) GetAutomationTasks(taskType string) ([]models.AutomationTask, error) {
	if a.automationService == nil {
		return []models.AutomationTask{}, nil
	}
	return a.automationService.GetTasks(taskType)
}

// GetAutomationTask 获取单个自动化任务
func (a *App) GetAutomationTask(id int) (*models.AutomationTask, error) {
	if a.automationService == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	return a.automationService.GetTask(id)
}

// CreateAutomationTask 创建自动化任务
func (a *App) CreateAutomationTask(name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool, slashCommand string) (int, error) {
	if a.automationService == nil {
		return 0, fmt.Errorf("自动化服务未初始化")
	}
	task := &models.AutomationTask{
		Name:             name,
		Description:      description,
		TaskType:         taskType,
		Config:           config,
		ScheduleType:     scheduleType,
		ScheduleConfig:   scheduleConfig,
		Enabled:          enabled,
		MaxRetries:       2,
		RetryIntervalSec: 30,
		SlashCommand:     slashCommand,
	}
	id, err := a.automationService.CreateTask(task)
	if err != nil {
		return 0, err
	}
	// 如果启用，加入调度
	if enabled && a.scheduler != nil {
		a.scheduler.ScheduleTask(id)
	}
	return id, nil
}

// UpdateAutomationTask 更新自动化任务
func (a *App) UpdateAutomationTask(id int, name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool, slashCommand string) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	task := &models.AutomationTask{
		ID:               id,
		Name:             name,
		Description:      description,
		TaskType:         taskType,
		Config:           config,
		ScheduleType:     scheduleType,
		ScheduleConfig:   scheduleConfig,
		Enabled:          enabled,
		MaxRetries:       2,
		RetryIntervalSec: 30,
		SlashCommand:     slashCommand,
	}
	err := a.automationService.UpdateTask(task)
	if err != nil {
		return err
	}
	// 重新调度
	if a.scheduler != nil {
		return a.scheduler.ScheduleTask(id)
	}
	return nil
}

// DeleteAutomationTask 删除自动化任务
func (a *App) DeleteAutomationTask(id int) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	if a.scheduler != nil {
		a.scheduler.UnscheduleTask(id)
	}
	return a.automationService.DeleteTask(id)
}

// ToggleAutomationTask 启用/禁用自动化任务
func (a *App) ToggleAutomationTask(id int, enabled bool) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	err := a.automationService.ToggleTask(id, enabled)
	if err != nil {
		return err
	}
	if a.scheduler == nil {
		return nil
	}
	if enabled {
		return a.scheduler.ScheduleTask(id)
	}
	a.scheduler.UnscheduleTask(id)
	return nil
}

// ExecuteTask 实现 core.TaskExecutor 接口（供 companionCore 斜杠命令调用）
func (a *App) ExecuteTask(taskID int) *models.AutomationExecution {
	if a.scheduler == nil {
		return &models.AutomationExecution{Status: "failed", ErrorMessage: "调度器未初始化"}
	}
	return a.scheduler.ExecuteTask(taskID)
}

// RunAutomationTaskNow 立即执行自动化任务（前端调用）
func (a *App) RunAutomationTaskNow(id int) (*models.AutomationExecution, error) {
	return a.ExecuteTask(id), nil
}

// GetAutomationExecutions 获取执行记录
func (a *App) GetAutomationExecutions(taskID int) ([]models.AutomationExecution, error) {
	if a.automationService == nil {
		return []models.AutomationExecution{}, nil
	}
	return a.automationService.GetExecutions(taskID)
}

// GetAutomationSteps 获取workflow步骤
func (a *App) GetAutomationSteps(taskID int) ([]models.AutomationStep, error) {
	if a.automationService == nil {
		return []models.AutomationStep{}, nil
	}
	return a.automationService.GetSteps(taskID)
}

// SaveAutomationSteps 保存workflow步骤
func (a *App) SaveAutomationSteps(taskID int, stepsJSON string) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	err := a.automationService.SaveStepsJSON(taskID, stepsJSON)
	if err != nil {
		return err
	}
	if a.scheduler != nil {
		return a.scheduler.ScheduleTask(taskID)
	}
	return nil
}

// GetStepExecutions 获取步骤执行详情
func (a *App) GetStepExecutions(executionID int) ([]models.StepExecution, error) {
	if a.automationService == nil {
		return []models.StepExecution{}, nil
	}
	return a.automationService.GetStepExecutions(executionID)
}

// GetAutomationDependencies 获取任务依赖关系
func (a *App) GetAutomationDependencies(taskID int) ([]models.AutomationDependency, error) {
	if a.automationService == nil {
		return []models.AutomationDependency{}, nil
	}
	return a.automationService.GetDependencies(taskID)
}

// GetAutomationDependents 获取依赖于指定任务的任务
func (a *App) GetAutomationDependents(taskID int) ([]models.AutomationDependency, error) {
	if a.automationService == nil {
		return []models.AutomationDependency{}, nil
	}
	return a.automationService.GetDependents(taskID)
}

// AddAutomationDependency 添加任务依赖
func (a *App) AddAutomationDependency(taskID, dependsOnID int, condition string) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	return a.automationService.AddDependency(taskID, dependsOnID, condition)
}

// RemoveAutomationDependency 删除任务依赖
func (a *App) RemoveAutomationDependency(id int) error {
	if a.automationService == nil {
		return fmt.Errorf("自动化服务未初始化")
	}
	return a.automationService.RemoveDependency(id)
}

// executeAutomationTask 执行业务逻辑（不再创建执行记录，由 scheduler 统一管理）
// - workflow 类型：读取保存的步骤，通过 Pipeline 按序执行
// - web_search 类型：使用 ResearchAgent 深度调研
// - 其他类型：使用 Orchestrator 规划执行
func (a *App) executeAutomationTask(execID int, task *models.AutomationTask) *models.AutomationExecution {
	startTime := time.Now()

	if a.companionCore == nil {
		return &models.AutomationExecution{ID: execID, Status: "failed", ErrorMessage: "系统未初始化"}
	}

	var status, resultType, content, filePath, errMsg string
	var err error

	switch task.TaskType {
	case "workflow":
		content, filePath, err = a.executeWorkflowTask(task, execID)
	case "web_search":
		content, filePath, err = a.executeDeepResearchTask(task)
	default:
		// agent_chat, report, reminder, reflection 等走 Orchestrator
		result, orchErr := a.companionCore.GetOrchestrator().Process(task.Description)
		if orchErr != nil {
			err = orchErr
		} else if result != nil {
			content = result.Content
		}
	}

	if err != nil {
		status = "failed"
		errMsg = err.Error()
	} else {
		status = "success"
		resultType = "text"
		if filePath != "" {
			resultType = "file"
		}
	}

	duration := time.Since(startTime).Milliseconds()

	return &models.AutomationExecution{
		ID:            execID,
		TaskID:        task.ID,
		Status:        status,
		ResultType:    resultType,
		ResultContent: content,
		ResultPath:    filePath,
		ErrorMessage:  errMsg,
		DurationMs:    duration,
	}
}

// executeWorkflowTask 执行 workflow 类型任务：读取已保存步骤 → 构建 Plan → Pipeline 逐步骤执行
func (a *App) executeWorkflowTask(task *models.AutomationTask, execID int) (content, filePath string, err error) {
	steps, err := a.automationService.GetSteps(task.ID)
	if err != nil || len(steps) == 0 {
		// 无步骤时回退到 Orchestrator
		result, orchErr := a.companionCore.GetOrchestrator().Process(task.Description)
		if orchErr != nil {
			return "", "", orchErr
		}
		if result != nil {
			return result.Content, "", nil
		}
		return "任务完成（无步骤）", "", nil
	}

	// 构建流水线步骤
	pipelineSteps := make([]pipeline.Step, 0, len(steps))
	for _, s := range steps {
		agentName, input := mapStepToAgent(s)
		// 用实时时间解析输入中的变量
		input = services.ReplaceVariables(input, nil)
		pipelineSteps = append(pipelineSteps, pipeline.Step{
			AgentName: agentName,
			Input:     input,
			OutputVar: s.OutputVar,
		})
	}

	plan := pipeline.Plan{Steps: pipelineSteps}
	runner := a.companionCore.GetOrchestrator().GetPipeline()

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	result := runner.Run(ctx, plan, nil, nil)
	if !result.Success {
		return "", "", fmt.Errorf("流水线执行失败: %s", result.Error)
	}

	// 提取最终内容
	content = result.Content
	for i := len(result.Steps) - 1; i >= 0; i-- {
		if result.Steps[i].Content != "" && result.Steps[i].Content != "(skipped)" {
			content = result.Steps[i].Content
			// 如果最后一步是文件生成，提取路径
			if steps[i].StepType == "file_generation" || steps[i].StepType == "save_file" {
				filePath = extractFilePath(result.Steps[i].Content, steps[i].Config)
			}
			break
		}
	}

	return content, filePath, nil
}

// mapStepToAgent 将步骤类型映射为 Agent 名称和输入内容
func mapStepToAgent(s models.AutomationStep) (agentName, input string) {
	cfg := parseSimpleConfig(s.Config)

	switch s.StepType {
	case "web_search", "search":
		// 使用 research agent 做深度调研
		agentName = "research"
		query, _ := cfg["query"].(string)
		if query == "" {
			query = s.Name
		}
		input = query
	case "summarize":
		agentName = "summarize"
		rawFrom, _ := cfg["use_raw_from"].(string)
		if rawFrom != "" {
			input = fmt.Sprintf("请对以下内容进行详细总结：\n{{%s}}", rawFrom)
		} else {
			input = "请对搜索到的内容进行结构化总结"
		}
	case "file_generation", "save_file":
		agentName = "file_generation"
		contentVar, _ := cfg["content_var"].(string)
		fp, _ := cfg["file_path"].(string)
		if contentVar != "" {
			input = fmt.Sprintf("将以下内容保存为文件：\n{{%s}}", contentVar)
		} else {
			input = "保存内容到文件"
		}
		if fp != "" {
			input += fmt.Sprintf("\n文件路径：%s", fp)
		}
	case "agent", "agent_chat":
		agentName, _ = cfg["agent_name"].(string)
		if agentName == "" {
			agentName = "web"
		}
		prompt, _ := cfg["prompt"].(string)
		input = prompt
		if input == "" {
			input = s.Name
		}
	case "notify":
		agentName = "emotion"
		notifyContent, _ := cfg["content"].(string)
		input = notifyContent
	default:
		agentName = "web"
		input = s.Name
	}

	if input == "" {
		input = s.Name
	}

	return agentName, input
}

// executeDeepResearchTask 使用 ResearchAgent 做深度联网调研（自动保存文件）
func (a *App) executeDeepResearchTask(task *models.AutomationTask) (content, filePath string, err error) {
	cfg := parseSimpleConfig(task.Config)
	query, _ := cfg["query"].(string)
	if query == "" {
		query = task.Description
	}
	if query == "" {
		query = task.Name
	}

	researchAgent := a.companionCore.GetResearchAgent()
	if researchAgent == nil {
		return "", "", fmt.Errorf("调研 Agent 未初始化")
	}

	// 注入实时时间 + 当前年份到查询，确保时效性
	now := time.Now()
	query = services.ReplaceVariables(query, nil)
	// 自动追加年份确保搜索结果的时效性
	if !strings.Contains(query, now.Format("2006")) {
		query = query + " " + now.Format("2006")
	}

	ctxAgent := agents.AgentContext{
		Content: query,
		History: []ai.Message{},
		Extra: map[string]interface{}{
			"source": "automation_task",
		},
	}

	result, err := researchAgent.Process(ctxAgent)
	if err != nil {
		return "", "", fmt.Errorf("调研执行失败: %w", err)
	}

	content = result.Content

	// 自动保存调研结果到文件
	researchDir := filepath.Join(a.dataDir, "research_docs")
	os.MkdirAll(researchDir, 0755)

	// 生成安全的文件名
	safeName := strings.Map(func(r rune) rune {
		if r == ' ' || r == '/' || r == '\\' || r == ':' {
			return '_'
		}
		return r
	}, strings.TrimSpace(task.Name))
	if len(safeName) > 30 {
		safeName = safeName[:30]
	}

	specifiedPath := filepath.Join(researchDir, fmt.Sprintf("%s_{{date}}_{{time}}.md", safeName))
	specifiedPath = services.ReplaceVariables(specifiedPath, nil)

	if a.companionCore.GetFileGenerationAgent() != nil {
		fileAgent := a.companionCore.GetFileGenerationAgent()
		fileCtx := agents.AgentContext{
			Content: content,
			History: []ai.Message{},
			Extra: map[string]interface{}{
				"raw_content": content,
				"title":       query,
				"template":    "research",
				"file_path":   specifiedPath,
			},
		}
		fileResult, fileErr := fileAgent.Process(fileCtx)
		if fileErr == nil && fileResult != nil {
			if dataMap, ok := fileResult.Data.(map[string]interface{}); ok {
				if fp, ok := dataMap["file_path"].(string); ok {
					filePath = fp
				}
			}
		}
		// 即使文件保存失败也不影响调研结果返回
	}

	return content, filePath, nil
}

// parseSimpleConfig 解析 JSON 配置为 map（容错）
// 解析失败时打印日志，避免上游拿到错误的"空配置"还误以为配置正确。
func parseSimpleConfig(configJSON string) map[string]interface{} {
	cfg := make(map[string]interface{})
	if configJSON == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		fmt.Printf("[parseSimpleConfig] 解析配置失败: %v (raw=%q)\n", err, configJSON)
		cfg = make(map[string]interface{})
	}
	return cfg
}

// extractFilePath 从步骤内容/配置中提取文件路径
func extractFilePath(content, configJSON string) string {
	cfg := parseSimpleConfig(configJSON)
	if fp, ok := cfg["file_path"].(string); ok && fp != "" {
		return fp
	}
	// 尝试从内容中提取路径
	if strings.Contains(content, "文件路径") {
		parts := strings.Split(content, "\n")
		for _, p := range parts {
			if strings.Contains(p, ".md") || strings.Contains(p, ".txt") {
				return strings.TrimSpace(p)
			}
		}
	}
	return ""
}

// GetTaskBySlashCommand 根据斜杠命令获取任务
func (a *App) GetTaskBySlashCommand(command string) (*models.AutomationTask, error) {
	if a.automationService == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	return a.automationService.GetTaskBySlashCommand(command)
}

// SlashCommandInfo 斜杠命令信息（前端指令菜单用）
// Cmd 形如 "/plan"；Kind 区分 "default"（内置提示词）和 "custom"（自动化任务）。
// Desc 为简短的描述，优先取任务的 description，否则取任务名。
type SlashCommandInfo struct {
	Cmd  string `json:"cmd"`
	Kind string `json:"kind"`
	Desc string `json:"desc"`
}

// GetAvailableSlashCommands 获取所有可用的斜杠命令：
//   - default：内置提示词命令（/plan、/review、/memory）
//   - custom：用户自己在自动化页面创建、带 slash_command 的任务
//
// 用于前端 ChatInput 的指令弹层，做到"用户新建/删除任务后命令菜单
// 实时同步"，避免硬编码遗漏。
func (a *App) GetAvailableSlashCommands() ([]SlashCommandInfo, error) {
	out := make([]SlashCommandInfo, 0, 8)

	// 1) 内置命令：与 companion_core.detectSlashCommand 的 switch 分支保持一致
	out = append(out,
		SlashCommandInfo{Cmd: "/plan", Kind: "default", Desc: "制定计划 / 设置目标"},
		SlashCommandInfo{Cmd: "/review", Kind: "default", Desc: "回顾复盘 / 总结"},
		SlashCommandInfo{Cmd: "/memory", Kind: "default", Desc: "查看记忆 / 回忆"},
	)

	// 2) 自定义命令：来自 automation_tasks 表，slash_command 非空且启用
	if a.automationService == nil {
		return out, nil
	}
	tasks, err := a.automationService.GetTasks("")
	if err != nil {
		// 出错时仍返回内置命令，不阻断前端
		fmt.Printf("[GetAvailableSlashCommands] 加载自动化任务失败: %v\n", err)
		return out, nil
	}
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		cmd := strings.TrimSpace(t.SlashCommand)
		if cmd == "" {
			continue
		}
		// 统一以 "/" 开头
		if !strings.HasPrefix(cmd, "/") {
			cmd = "/" + cmd
		}
		// 与内置命令重名时让自定义覆盖（用户主动配置应优先于默认）
		desc := strings.TrimSpace(t.Description)
		if desc == "" {
			desc = t.Name
		}
		out = append(out, SlashCommandInfo{
			Cmd:  cmd,
			Kind: "custom",
			Desc: desc,
		})
	}
	return out, nil
}

// GetTaskConfigSchema 获取任务类型配置表单Schema
func (a *App) GetTaskConfigSchema(taskType string) ([]models.ConfigField, error) {
	return getTaskSchema(taskType), nil
}

// GetAllTaskSchemas 获取所有任务类型的Schema
func (a *App) GetAllTaskSchemas() map[string][]models.ConfigField {
	return getAllTaskSchemas()
}

// getTaskSchema 返回指定类型的配置表单字段
func getTaskSchema(taskType string) []models.ConfigField {
	schemas := getAllTaskSchemas()
	if s, ok := schemas[taskType]; ok {
		return s
	}
	return nil
}

// getAllTaskSchemas 返回所有任务类型的配置表单字段
func getAllTaskSchemas() map[string][]models.ConfigField {
	return map[string][]models.ConfigField{
		"agent_chat": {
			{Key: "agent_name", Label: "选择Agent", Type: "select", Required: true},
			{Key: "prompt", Label: "提示词", Type: "textarea", Required: true},
		},
		"web_search": {
			{Key: "query", Label: "搜索关键词", Type: "text", Required: true},
			{Key: "need_summary", Label: "AI总结", Type: "boolean", Default: "true"},
		},
		"reminder": {
			{Key: "content", Label: "提醒内容", Type: "textarea", Required: true},
		},
		"workflow": {
			{Key: "_notice", Label: "提示", Type: "text", Placeholder: "流程步骤请在步骤管理页面配置"},
		},
	}
}

// ==================== 信息整合与文档生成 ====================

// SummarizeContent 信息整合接口
// summaryType: "brief"（简报）/ "detailed"（详报）/ "technical"（技术报告）
// rawContent 为空时自动搜索，不为空时直接整理给定文本
func (a *App) SummarizeContent(topic, rawContent, summaryType string) (map[string]interface{}, error) {
	if a.companionCore == nil {
		return nil, fmt.Errorf("系统未初始化")
	}

	agent := a.companionCore.GetSummarizeAgent()
	if agent == nil {
		return nil, fmt.Errorf("信息整合 Agent 未初始化")
	}

	content := topic
	if content == "" {
		content = rawContent
	}

	ctx := agents.AgentContext{
		Content: content,
		History: []ai.Message{},
		Extra: map[string]interface{}{
			"query":        topic,
			"raw_content":  rawContent,
			"summary_type": summaryType,
		},
	}

	result, err := agent.Process(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"content": result.Content,
		"emotion": result.Emotion,
		"data":    result.Data,
	}, nil
}

// GenerateDocument 文档生成接口
// template: "research" / "weekly" / "meeting" / "tech_review" / "general"
func (a *App) GenerateDocument(title, content, template string) (map[string]interface{}, error) {
	if a.companionCore == nil {
		return nil, fmt.Errorf("系统未初始化")
	}

	agent := a.companionCore.GetFileGenerationAgent()
	if agent == nil {
		return nil, fmt.Errorf("文件生成 Agent 未初始化")
	}

	if template == "" {
		template = "general"
	}

	ctx := agents.AgentContext{
		Content: content,
		History: []ai.Message{},
		Extra: map[string]interface{}{
			"raw_content": content,
			"title":       title,
			"template":    template,
		},
	}

	result, err := agent.Process(ctx)
	if err != nil {
		return nil, err
	}

	filePath := ""
	if result.Data != nil {
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			if fp, ok := dataMap["file_path"].(string); ok {
				filePath = fp
			}
		}
	}

	return map[string]interface{}{
		"content":   result.Content,
		"file_path": filePath,
	}, nil
}

// GetResearchDocs 获取已生成的研究文档列表
func (a *App) GetResearchDocs() ([]map[string]interface{}, error) {
	docsDir := filepath.Join(a.dataDir, "research_docs")
	entries, err := os.ReadDir(docsDir)
	if err != nil {
		return []map[string]interface{}{}, nil
	}

	var docs []map[string]interface{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".md" {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		docs = append(docs, map[string]interface{}{
			"name":     entry.Name(),
			"path":     filepath.Join(docsDir, entry.Name()),
			"size":     info.Size(),
			"mod_time": info.ModTime().Format("2006-01-02 15:04:05"),
		})
	}

	return docs, nil
}

// ReadResearchDoc 读取研究文档内容
func (a *App) ReadResearchDoc(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ==================== 对话话题建议 ====================

// GetTopicSuggestions 获取对话话题建议
func (a *App) GetTopicSuggestions() ([]string, error) {
	suggestions := []string{
		"查看今日待办任务",
		"回顾最近的对话记录",
		"梳理当前项目进展",
		"总结最近的学习情况",
		"制定一个新的计划",
		"搜索某个技术问题",
		"整理最近的记忆",
		"复盘这段时间的进展",
	}

	// 根据记忆个性化话题
	// 注意：记忆模型中类型为 L1/L2/L3/L4/L5，不是 "L4-PLAN"。
	// 这里匹配 L4（项目目标）以及它的常见中文化别名。
	if a.memory != nil {
		mems, err := a.memory.GetKeyMemories(5)
		if err == nil && len(mems) > 0 {
			for _, m := range mems {
				if m.Type == "L4" || m.Type == "L4-PLAN" || m.Type == "L4-计划目标" {
					suggestions = append([]string{"跟进「" + m.Content + "」的进展"}, suggestions...)
					break
				}
			}
		}
	}

	// 根据当前时间调整话题
	hour := time.Now().Hour()
	if hour < 9 {
		suggestions = append([]string{"查看今天的任务安排"}, suggestions...)
	} else if hour >= 22 {
		suggestions = append([]string{"总结今天完成的事项"}, suggestions...)
	}

	return suggestions, nil
}
