package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/core"
	"ai-companion/internal/db"
	"ai-companion/internal/models"
	"ai-companion/internal/scheduler"
	"ai-companion/internal/services"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	_ "github.com/mattn/go-sqlite3"
)

// App 应用主结构
type App struct {
	ctx              context.Context
	db               *sql.DB
	aiClient         *ai.Client
	companionCore    *core.CompanionCore
	settings         *services.SettingsService
	memory           *services.MemoryService
	conversation     *services.ConversationService
	plan               *services.PlanService
	automationService  *services.AutomationService
	scheduler          *scheduler.Scheduler
	dataDir            string
	shutdownOnce     sync.Once
	isShuttingDown   bool
	mu               sync.Mutex
}

// NewApp 创建新应用实例
func NewApp() (*App, error) {
	app := &App{}

	// 初始化数据目录
	fmt.Println("正在初始化数据目录...")
	appDataDir, err := app.getDataDir()
	if err != nil {
		return nil, fmt.Errorf("无法获取数据目录: %w", err)
	}
	app.dataDir = appDataDir
	fmt.Println("数据目录:", app.dataDir)

	// 创建必要的子目录
	app.ensureDirectories()

	// 初始化数据库
	fmt.Println("正在初始化数据库...")
	database, err := db.InitDB(filepath.Join(app.dataDir, "companion.db"))
	if err != nil {
		return nil, fmt.Errorf("数据库初始化失败: %w", err)
	}
	app.db = database
	fmt.Println("数据库连接成功")

	// 初始化服务
	fmt.Println("正在初始化服务...")
	app.settings = services.NewSettingsService(app.db)
	if app.settings == nil {
		return nil, fmt.Errorf("设置服务创建失败")
	}
	app.memory = services.NewMemoryService(app.db)
	app.conversation = services.NewConversationService(app.db)
	app.conversation.SetConversationsDir(filepath.Join(app.dataDir, "conversations"))
	app.plan = services.NewPlanService(app.db)
	app.automationService = services.NewAutomationService(app.db)
	fmt.Println("服务初始化成功")

	// 初始化默认设置
	fmt.Println("正在初始化默认设置...")
	if err := app.settings.InitDefaults(); err != nil {
		return nil, fmt.Errorf("初始化默认设置失败: %w", err)
	}
	fmt.Println("默认设置初始化完成")

	// 初始化系统任务模板（不依赖自动化引擎，确保模板一定存在）
	fmt.Println("正在初始化任务模板...")
	app.initTaskTemplates()
	fmt.Println("任务模板初始化完成")

	// 初始化 AI 客户端（支持多 provider）
	fmt.Println("正在初始化 AI 客户端...")
	app.initAIClient()
	fmt.Println("AI 客户端初始化完成")

	fmt.Println("Along 核心初始化完成")
	return app, nil
}

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("应用启动时发生 panic: %v\n", r)
		}
	}()

	// 初始化 Companion Core
	if a.aiClient != nil && a.memory != nil && a.conversation != nil && a.plan != nil {
		a.companionCore = core.NewCompanionCore(a.aiClient, a.memory, a.conversation, a.plan)

		// 设置文件生成 Agent 的输出目录
		if fileAgent := a.companionCore.GetFileGenerationAgent(); fileAgent != nil {
			fileAgent.SetOutputDir(filepath.Join(a.dataDir, "research_docs"))
		}
	}

	// 初始化调度器
	if a.db != nil && a.companionCore != nil {
		a.scheduler = scheduler.New(a.db, a.dataDir)
		// 设置 Agent 任务执行回调
		a.scheduler.OnExecuteAgentTask = func(task *models.AutomationTask) *models.AutomationExecution {
			return a.executeAutomationTask(task)
		}
		if err := a.scheduler.Start(); err != nil {
			fmt.Println("调度器启动失败:", err)
		}

		// 设置自动化服务到 companionCore（用于斜杠命令调用）
		a.companionCore.SetAutomationService(a.automationService)
		a.companionCore.SetTaskExecutor(a)

		// 初始化默认的AI前沿知识调研任务
		a.initDefaultResearchTask()
	}

	// 注册设置变更钩子
	a.setupSettingHooks()

	// 同步开机启动设置（仅 Windows）
	a.syncAutoStart()

	// 启动系统托盘（如果启用）
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

	fmt.Println("Along 已启动")
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
func (a *App) domReady(ctx context.Context) {
	// 可以在这里触发前端初始化事件
}

// beforeClose 关闭前钩子：返回 true 阻止关闭，返回 false 允许关闭
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	a.mu.Lock()
	if a.isShuttingDown {
		a.mu.Unlock()
		return false
	}
	a.mu.Unlock()

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
	case "tray":
	default:
		if IsTrayRunning() {
			wruntime.Hide(ctx)
			return true
		}
	}
	return false
}

// QuitApp 完全退出应用（供前端调用）
func (a *App) QuitApp() {
	a.shutdownOnce.Do(func() {
		fmt.Println("Along 正在退出...")

		a.mu.Lock()
		a.isShuttingDown = true
		a.mu.Unlock()

		if a.scheduler != nil {
			a.scheduler.Stop()
			a.scheduler = nil
		}

		StopTray()

		if a.aiClient != nil {
			a.aiClient.Close()
			a.aiClient = nil
		}

		if a.db != nil {
			a.db.Close()
			a.db = nil
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

// initDefaultResearchTask 初始化默认的AI前沿知识调研任务
func (a *App) initDefaultResearchTask() {
	if a.scheduler == nil {
		fmt.Println("自动化引擎未初始化，跳过默认任务创建")
		return
	}
	a.ensureDefaultTasks()
}

// ensureDefaultTasks 确保默认任务存在
func (a *App) ensureDefaultTasks() {
	researchDir := filepath.Join(a.dataDir, "research_docs")
	os.MkdirAll(researchDir, 0755)

	var existingID int
	var existingName string
	err := a.db.QueryRow(`SELECT id, name FROM automation_tasks WHERE slash_command = ?`, "/research").Scan(&existingID, &existingName)

	config := map[string]interface{}{
		"query":        "AI前沿技术 2026 research paper arXiv 大模型最新进展 Loop AI Agentic AI 多智能体系统 推理优化 学术研究",
		"engine":       "duckduckgo",
		"result_count": 15,
		"need_summary": true,
		"output_type":  "file",
		"file_path":    filepath.Join(researchDir, "ai_research_{{date}}.md"),
	}
	configJSON, _ := json.Marshal(config)

	scheduleConfig := map[string]interface{}{
		"day":  1,
		"time": "09:00",
	}
	scheduleConfigJSON, _ := json.Marshal(scheduleConfig)

	var taskID int
	if err == nil && existingID > 0 {
		task := &models.AutomationTask{
			ID:               existingID,
			Name:             "AI前沿知识调研",
			Description:      "每周一上午9点自动联网搜索AI最新技术进展，AI总结后保存为文档",
			TaskType:         "web_search",
			Config:           string(configJSON),
			ScheduleType:     "weekly",
			ScheduleConfig:   string(scheduleConfigJSON),
			Enabled:          true,
			MaxRetries:       2,
			RetryIntervalSec: 30,
			SlashCommand:     "/research",
		}
		err = a.automationService.UpdateTask(task)
		if err != nil {
			fmt.Println("更新调研任务失败:", err)
			return
		}
		taskID = existingID
		fmt.Println("已更新默认任务: AI前沿知识调研")
	} else {
		task := &models.AutomationTask{
			Name:             "AI前沿知识调研",
			Description:      "每周一上午9点自动联网搜索AI最新技术进展，AI总结后保存为文档",
			TaskType:         "web_search",
			Config:           string(configJSON),
			ScheduleType:     "weekly",
			ScheduleConfig:   string(scheduleConfigJSON),
			Enabled:          true,
			MaxRetries:       2,
			RetryIntervalSec: 30,
			SlashCommand:     "/research",
		}
		taskID, err = a.automationService.CreateTask(task)
		if err != nil {
			fmt.Println("创建默认调研任务失败:", err)
			return
		}
		fmt.Println("已创建默认任务: AI前沿知识调研")
	}

	a.scheduler.ScheduleTask(taskID)
}

// initTaskTemplates 初始化系统任务模板（7个最终版模板）
func (a *App) initTaskTemplates() {
	researchDir := filepath.Join(a.dataDir, "research_docs")
	os.MkdirAll(researchDir, 0755)

	researchPath := filepath.Join(researchDir, "research_{{date}}.md")

	templates := []struct {
		Name                  string
		Icon                  string
		Description           string
		TaskType              string
		DefaultConfig         string
		DefaultScheduleType   string
		DefaultScheduleConfig string
		Steps                 string
	}{
		{
			Name:                  "联网调研",
			Icon:                  "🔍",
			Description:           "搜索(web) → 总结(summarize) → 保存文件(file_generation)，定期搜索最新信息并生成报告",
			TaskType:              "workflow",
			DefaultConfig:         `{}`,
			DefaultScheduleType:   "weekly",
			DefaultScheduleConfig: `{"day": 1, "time": "09:00"}`,
			Steps: `[
				{"step_type": "web_search", "name": "搜索", "config": {"query": "", "result_count": 10}, "output_var": "search_results"},
				{"step_type": "summarize", "name": "总结", "config": {"summary_type": "detailed", "use_raw_from": "search_results"}, "output_var": "summary"},
				{"step_type": "file_generation", "name": "保存文件", "config": {"file_path": "` + researchPath + `", "content_var": "summary"}}
			]`,
		},
		{
			Name:                  "周报总结",
			Icon:                  "📝",
			Description:           "获取本周对话 → 获取本周任务 → 总结(summarize) → 保存文件，基于对话和任务生成周报",
			TaskType:              "workflow",
			DefaultConfig:         `{}`,
			DefaultScheduleType:   "weekly",
			DefaultScheduleConfig: `{"day": 5, "time": "18:00"}`,
			Steps: `[
				{"step_type": "agent_chat", "name": "生成本周总结报告", "config": {"prompt": "请帮我生成本周的总结报告，内容包括：本周对话摘要、计划进度回顾、里程碑完成情况、打卡记录统计、下周建议。请结合本周的所有对话内容来生成。"}, "output_var": "report"}
			]`,
		},
		{
			Name:                  "数据备份",
			Icon:                  "💾",
			Description:           "定期备份数据库",
			TaskType:              "backup",
			DefaultConfig:         `{}`,
			DefaultScheduleType:   "daily",
			DefaultScheduleConfig: `{"time": "23:00"}`,
			Steps:                 "[]",
		},
		{
			Name:                  "每日提醒",
			Icon:                  "🔔",
			Description:           "定时消息推送",
			TaskType:              "reminder",
			DefaultConfig:         `{"message": "该休息一下了！"}`,
			DefaultScheduleType:   "daily",
			DefaultScheduleConfig: `{"time": "10:00"}`,
			Steps:                 "[]",
		},
		{
			Name:                  "习惯打卡统计",
			Icon:                  "✅",
			Description:           "打卡 → 总结 → 通知，统计打卡情况",
			TaskType:              "habit_checkin",
			DefaultConfig:         `{}`,
			DefaultScheduleType:   "daily",
			DefaultScheduleConfig: `{"time": "22:00"}`,
			Steps:                 "[]",
		},
		{
			Name:                  "反思复盘",
			Icon:                  "🧠",
			Description:           "基于对话和记忆进行定期复盘",
			TaskType:              "reflection",
			DefaultConfig:         `{"period": "week"}`,
			DefaultScheduleType:   "weekly",
			DefaultScheduleConfig: `{"day": 0, "time": "20:00"}`,
			Steps:                 "[]",
		},
		{
			Name:                  "自定义工作流",
			Icon:                  "⚙️",
			Description:           "从头设计流程",
			TaskType:              "workflow",
			DefaultConfig:         `{}`,
			DefaultScheduleType:   "manual",
			DefaultScheduleConfig: "{}",
			Steps:                 "[]",
		},
	}

	for _, t := range templates {
		var existingID int
		err := a.db.QueryRow(`SELECT id FROM task_templates WHERE name = ? AND is_system = 1`, t.Name).Scan(&existingID)
		if err == nil && existingID > 0 {
			continue
		}

		_, err = a.db.Exec(`INSERT INTO task_templates (name, icon, description, task_type, default_config, default_schedule_type, default_schedule_config, steps, is_system) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)`,
			t.Name, t.Icon, t.Description, t.TaskType, t.DefaultConfig, t.DefaultScheduleType, t.DefaultScheduleConfig, t.Steps)
		if err != nil {
			fmt.Printf("创建模板失败 %s: %v\n", t.Name, err)
		}
	}

	fmt.Println("已初始化系统任务模板（7个）")
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
		if err == nil && fullReply != "" {
			go NotifyNewMessage(fullReply)
		}
		if err != nil {
			wruntime.EventsEmit(a.ctx, "chat-stream", map[string]interface{}{
				"error": err.Error(),
				"done":  true,
			})
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
		if err == nil && fullReply != "" {
			go NotifyNewMessage(fullReply)
		}
		if err != nil {
			wruntime.EventsEmit(a.ctx, "chat-stream", map[string]interface{}{
				"conversation_id": conversationID,
				"error":           err.Error(),
				"done":            true,
			})
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
		if a.aiClient != nil && newValue != "" {
			a.aiClient.SetAPIKey(newValue)
		}
		return nil
	})

	// API Provider 变更时重新初始化客户端
	a.settings.OnChange("api_provider", func(key, oldValue, newValue string) error {
		if newValue == "" {
			return nil
		}
		apiKey, _ := a.settings.Get("api_key")
		if a.aiClient != nil {
			a.aiClient.SetProvider(newValue)
		} else {
			a.aiClient = ai.NewClient(newValue, apiKey)
		}
		if a.companionCore != nil {
			a.companionCore.UpdateAIClient(a.aiClient)
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
	_, err := a.db.Exec(
		"INSERT INTO highlights (title, description, date, user_marked, created_at) VALUES (?, ?, ?, 1, datetime('now'))",
		title, description, date,
	)
	return err
}

// DeleteHighlight 删除高光回忆
func (a *App) DeleteHighlight(id int) error {
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

	memories, mErr := a.memory.SearchMemories(query)
	if mErr != nil {
		memories = []models.Memory{}
	}
	messages, cErr := a.conversation.SearchMessages(query, 20)
	if cErr != nil {
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
	if a.scheduler == nil {
		return []models.AutomationTask{}, nil
	}
	return a.automationService.GetTasks(taskType)
}

// GetAutomationTask 获取单个自动化任务
func (a *App) GetAutomationTask(id int) (*models.AutomationTask, error) {
	if a.scheduler == nil {
		return nil, fmt.Errorf("自动化引擎未初始化")
	}
	return a.automationService.GetTask(id)
}

// CreateAutomationTask 创建自动化任务
func (a *App) CreateAutomationTask(name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool, slashCommand string) (int, error) {
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
	if enabled {
		a.scheduler.ScheduleTask(id)
	}
	return id, nil
}

// UpdateAutomationTask 更新自动化任务
func (a *App) UpdateAutomationTask(id int, name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool, slashCommand string) error {
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
	return a.scheduler.ScheduleTask(id)
}

// DeleteAutomationTask 删除自动化任务
func (a *App) DeleteAutomationTask(id int) error {
	a.scheduler.UnscheduleTask(id)
	return a.automationService.DeleteTask(id)
}

// ToggleAutomationTask 启用/禁用自动化任务
func (a *App) ToggleAutomationTask(id int, enabled bool) error {
	err := a.automationService.ToggleTask(id, enabled)
	if err != nil {
		return err
	}
	if enabled {
		return a.scheduler.ScheduleTask(id)
	}
	a.scheduler.UnscheduleTask(id)
	return nil
}

// ExecuteTask 实现 core.TaskExecutor 接口（供 companionCore 调用）
func (a *App) ExecuteTask(taskID int) *models.AutomationExecution {
	return a.scheduler.ExecuteTask(taskID)
}

// RunAutomationTaskNow 立即执行自动化任务
func (a *App) RunAutomationTaskNow(id int) (*models.AutomationExecution, error) {
	exec := a.scheduler.ExecuteTask(id)
	if exec == nil {
		return nil, fmt.Errorf("执行失败")
	}
	return exec, nil
}

// GetAutomationExecutions 获取执行记录
func (a *App) GetAutomationExecutions(taskID int) ([]models.AutomationExecution, error) {
	return a.automationService.GetExecutions(taskID)
}

// GetAutomationSteps 获取workflow步骤
func (a *App) GetAutomationSteps(taskID int) ([]models.AutomationStep, error) {
	return a.automationService.GetSteps(taskID)
}

// SaveAutomationSteps 保存workflow步骤
func (a *App) SaveAutomationSteps(taskID int, stepsJSON string) error {
	err := a.automationService.SaveStepsJSON(taskID, stepsJSON)
	if err != nil {
		return err
	}
	return a.scheduler.ScheduleTask(taskID)
}

// GetStepExecutions 获取步骤执行详情
func (a *App) GetStepExecutions(executionID int) ([]models.StepExecution, error) {
	return a.automationService.GetStepExecutions(executionID)
}

// GetAutomationDependencies 获取任务依赖关系
func (a *App) GetAutomationDependencies(taskID int) ([]models.AutomationDependency, error) {
	return a.automationService.GetDependencies(taskID)
}

// GetAutomationDependents 获取依赖于指定任务的任务
func (a *App) GetAutomationDependents(taskID int) ([]models.AutomationDependency, error) {
	return a.automationService.GetDependents(taskID)
}

// AddAutomationDependency 添加任务依赖
func (a *App) AddAutomationDependency(taskID, dependsOnID int, condition string) error {
	return a.automationService.AddDependency(taskID, dependsOnID, condition)
}

// RemoveAutomationDependency 删除任务依赖
func (a *App) RemoveAutomationDependency(id int) error {
	return a.automationService.RemoveDependency(id)
}

// GetTaskTemplates 获取任务模板列表
func (a *App) GetTaskTemplates() ([]models.TaskTemplate, error) {
	if a.automationService == nil {
		return []models.TaskTemplate{}, nil
	}
	return a.automationService.GetTaskTemplates()
}

// GetTaskTemplate 获取单个模板
func (a *App) GetTaskTemplate(id int) (*models.TaskTemplate, error) {
	if a.automationService == nil {
		return nil, fmt.Errorf("自动化服务未初始化")
	}
	return a.automationService.GetTaskTemplate(id)
}

// executeAutomationTask 执行自动化任务（通过 Orchestrator）
func (a *App) executeAutomationTask(task *models.AutomationTask) *models.AutomationExecution {
	startTime := time.Now()
	execID, err := a.automationService.CreateExecution(task.ID)
	if err != nil {
		return &models.AutomationExecution{Status: "failed", ErrorMessage: err.Error()}
	}

	if a.companionCore == nil {
		a.automationService.UpdateExecution(execID, "failed", "none", "", "", "系统未初始化", 0)
		return &models.AutomationExecution{ID: execID, Status: "failed"}
	}

	// 使用 Orchestrator 规划并执行
	result, err := a.companionCore.GetOrchestrator().Process(task.Description)
	status := "success"
	errMsg := ""
	content := ""
	if err != nil {
		status = "failed"
		errMsg = err.Error()
	}
	if result != nil {
		content = result.Content
	}

	duration := time.Since(startTime).Milliseconds()
	a.automationService.UpdateExecution(execID, status, "text", content, "", errMsg, duration)

	return &models.AutomationExecution{
		ID:            execID,
		TaskID:        task.ID,
		Status:        status,
		ResultContent: content,
		ErrorMessage:  errMsg,
		DurationMs:    duration,
	}
}

// CreateTaskFromTemplate 从模板创建任务
func (a *App) CreateTaskFromTemplate(templateID int, name, description string, scheduleType, scheduleConfig, slashCommand string) (int, error) {
	id, err := a.automationService.CreateTaskFromTemplate(templateID, name, description, scheduleType, scheduleConfig, slashCommand)
	if err != nil {
		return 0, err
	}
	a.scheduler.ScheduleTask(id)
	return id, nil
}

// GetTaskBySlashCommand 根据斜杠命令获取任务
func (a *App) GetTaskBySlashCommand(command string) (*models.AutomationTask, error) {
	return a.automationService.GetTaskBySlashCommand(command)
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
	mems, err := a.memory.GetKeyMemories(5)
	if err == nil && len(mems) > 0 {
		for _, m := range mems {
			if m.Type == "L4-PLAN" || m.Type == "L4-计划目标" {
				suggestions = append([]string{"跟进「" + m.Content + "」的进展"}, suggestions...)
				break
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
