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

	"ai-companion/internal/ai"
	"ai-companion/internal/agents"
	"ai-companion/internal/automation"
	"ai-companion/internal/core"
	"ai-companion/internal/db"
	"ai-companion/internal/models"
	"ai-companion/internal/services"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	_ "github.com/mattn/go-sqlite3"
)

// App 应用主结构
type App struct {
	ctx             context.Context
	db              *sql.DB
	aiClient        *ai.Client
	companionCore   *core.CompanionCore
	settings        *services.SettingsService
	memory          *services.MemoryService
	conversation    *services.ConversationService
	plan            *services.PlanService
	automationEngine *automation.Engine
	dataDir         string
	shutdownOnce    sync.Once
	isShuttingDown  bool
	mu              sync.Mutex
}

// NewApp 创建新应用实例
func NewApp() *App {
	return &App{}
}

// startup 应用启动时调用
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 初始化数据目录
	appDataDir, err := a.getDataDir()
	if err != nil {
		fmt.Println("无法获取数据目录:", err)
		return
	}
	a.dataDir = appDataDir

	// 创建必要的子目录
	a.ensureDirectories()

	// 初始化数据库
	database, err := db.InitDB(filepath.Join(a.dataDir, "companion.db"))
	if err != nil {
		fmt.Println("数据库初始化失败:", err)
		return
	}
	a.db = database

	// 初始化服务
	a.settings = services.NewSettingsService(a.db)
	a.memory = services.NewMemoryService(a.db)
	a.conversation = services.NewConversationService(a.db)
	a.conversation.SetConversationsDir(filepath.Join(a.dataDir, "conversations"))
	a.plan = services.NewPlanService(a.db)

	// 初始化 AI 客户端（支持多 provider）
	a.initAIClient()

	// 初始化 Companion Core
	a.companionCore = core.NewCompanionCore(a.aiClient, a.memory, a.conversation, a.plan)

	// 设置文件生成 Agent 的输出目录
	if fileAgent := a.companionCore.GetFileGenerationAgent(); fileAgent != nil {
		fileAgent.SetOutputDir(filepath.Join(a.dataDir, "research_docs"))
	}

	// 初始化自动化引擎
	a.automationEngine = automation.NewEngine(a.db, a.companionCore.GetAgentManager(), a.dataDir)
	if err := a.automationEngine.Start(); err != nil {
		fmt.Println("自动化引擎启动失败:", err)
	}

	// 同步开机启动设置（仅 Windows）
	a.syncAutoStart()

	// 启动系统托盘（如果启用）
	trayEnabled := true
	trayVal, _ := a.settings.Get("system_tray_enabled")
	if trayVal == "false" || trayVal == "0" {
		trayEnabled = false
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

	fmt.Println("AI Companion 已启动")
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
		fmt.Println("AI Companion 正在退出...")

		a.mu.Lock()
		a.isShuttingDown = true
		a.mu.Unlock()

		if a.automationEngine != nil {
			a.automationEngine.Stop()
			a.automationEngine = nil
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

		fmt.Println("AI Companion 已退出")
	})
}

// shutdown 应用关闭时调用
func (a *App) shutdown(ctx context.Context) {
	a.QuitApp()
}

// getDataDir 获取数据存储目录
func (a *App) getDataDir() (string, error) {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		exe, err := os.Executable()
		if err != nil {
			return "", err
		}
		return filepath.Dir(exe), nil
	}
	dir := filepath.Join(appData, "AICompanion")
	return dir, nil
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
	return a.conversation.CreateConversation(title)
}

// ListConversations 获取所有对话列表
func (a *App) ListConversations() ([]models.Conversation, error) {
	return a.conversation.ListConversations()
}

// GetConversation 获取单个对话
func (a *App) GetConversation(id int) (*models.Conversation, error) {
	return a.conversation.GetConversation(id)
}

// RenameConversation 重命名对话
func (a *App) RenameConversation(id int, title string) error {
	return a.conversation.RenameConversation(id, title)
}

// DeleteConversation 删除对话
func (a *App) DeleteConversation(id int) error {
	return a.conversation.DeleteConversation(id)
}

// GetConversationMessages 获取指定对话的消息
func (a *App) GetConversationMessages(conversationID int) ([]models.Message, error) {
	return a.conversation.GetMessagesByConversationID(conversationID)
}

// GetConversationHistory 获取对话历史（兼容旧接口：按日期）
func (a *App) GetConversationHistory(date string) ([]models.Message, error) {
	return a.conversation.GetMessages(date)
}

// GetConversationDates 获取有对话的所有日期（用于月度总结与历史选择）
func (a *App) GetConversationDates() ([]string, error) {
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
	return a.memory.GetMemories(memoryType)
}

// GetMemoryCountByType 按类型统计记忆数量
func (a *App) GetMemoryCountByType() (map[string]int, error) {
	return a.memory.GetCountByType()
}

// UpdateMemory 更新记忆
func (a *App) UpdateMemory(id int, content string) error {
	return a.memory.UpdateMemory(id, content)
}

// DeleteMemory 删除记忆
func (a *App) DeleteMemory(id int) error {
	return a.memory.DeleteMemory(id)
}

// AddMemory 添加记忆（前端调用）
func (a *App) AddMemory(memoryType, content, source string, confidence float64) error {
	return a.memory.AddMemory(memoryType, content, source, confidence)
}

// ==================== 计划相关 ====================

// GetGoals 获取所有计划
func (a *App) GetGoals() ([]models.Goal, error) {
	return a.plan.GetAllGoals()
}

// GetGoalsByType 按类型获取计划
func (a *App) GetGoalsByType(goalType string) ([]models.Goal, error) {
	return a.plan.GetGoalsByType(goalType)
}

// CreateGoal 创建计划
func (a *App) CreateGoal(title, description, goalType string) (*models.Goal, error) {
	return a.plan.CreateGoal(title, description, goalType)
}

// UpdateGoal 更新计划
func (a *App) UpdateGoal(id int, title, description, status, currentFocus, nextStep, mood string, progress int) error {
	return a.plan.UpdateGoal(id, title, description, status, currentFocus, nextStep, mood, progress)
}

// DeleteGoal 删除计划
func (a *App) DeleteGoal(id int) error {
	return a.plan.DeleteGoal(id)
}

// GetMilestones 获取计划的里程碑
func (a *App) GetMilestones(goalID int) ([]models.Milestone, error) {
	return a.plan.GetMilestones(goalID)
}

// AddMilestone 添加里程碑
func (a *App) AddMilestone(goalID int, title, description string) (*models.Milestone, error) {
	return a.plan.AddMilestone(goalID, title, description)
}

// UpdateMilestone 更新里程碑
func (a *App) UpdateMilestone(id int, title, description, status string) error {
	return a.plan.UpdateMilestone(id, title, description, status)
}

// CompleteMilestone 完成里程碑
func (a *App) CompleteMilestone(id int, companionComment string) error {
	return a.plan.CompleteMilestone(id, companionComment)
}

// DeleteMilestone 删除里程碑
func (a *App) DeleteMilestone(id int) error {
	return a.plan.DeleteMilestone(id)
}

// GetCheckIns 获取计划的记录
func (a *App) GetCheckIns(goalID int) ([]models.CheckIn, error) {
	return a.plan.GetCheckIns(goalID)
}

// AddCheckIn 添加记录
func (a *App) AddCheckIn(goalID int, content, mood, companionResponse string) (*models.CheckIn, error) {
	return a.plan.AddCheckIn(goalID, content, mood, companionResponse)
}

// DeleteCheckIn 删除记录
func (a *App) DeleteCheckIn(id int) error {
	return a.plan.DeleteCheckIn(id)
}

// SearchGoals 搜索计划
func (a *App) SearchGoals(keyword string) ([]models.Goal, error) {
	return a.plan.SearchGoals(keyword)
}

// ==================== 设置相关 ====================

// GetSettings 获取设置
func (a *App) GetSettings() (map[string]string, error) {
	return a.settings.GetAll()
}

// SaveSetting 保存设置
func (a *App) SaveSetting(key, value string) error {
	err := a.settings.Set(key, value)
	if err != nil {
		return err
	}
	// 如果保存的是 API Key，更新 AI 客户端
	if key == "api_key" && value != "" {
		if a.aiClient != nil {
			a.aiClient.SetAPIKey(value)
		}
	}
	// 如果保存的是 api_provider，重新初始化客户端
	if key == "api_provider" && value != "" {
		apiKey, _ := a.settings.Get("api_key")
		if a.aiClient != nil {
			a.aiClient.SetProvider(value)
		} else {
			a.aiClient = ai.NewClient(value, apiKey)
		}
		// 重建 CompanionCore 让它使用新 client
		if a.companionCore != nil {
			a.companionCore.UpdateAIClient(a.aiClient)
		}
	}
	// 开机启动
	if key == "auto_start" {
		enabled := value == "true" || value == "1"
		if err := SetAutoStart(enabled); err != nil {
			fmt.Println("设置开机启动失败:", err)
		}
	}
	// 系统托盘
	if key == "system_tray_enabled" {
		// 只在禁用时停止托盘，启用需要重启应用生效
		if value == "false" || value == "0" {
			StopTray()
		}
	}
	return nil
}

// ==================== 观察与复盘 ====================

// GetObservations 获取观察列表
func (a *App) GetObservations() ([]models.Observation, error) {
	return a.memory.GetObservations()
}

// GenerateReflection 生成复盘
func (a *App) GenerateReflection(period string) (*models.Reflection, error) {
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
	exportPath := filepath.Join(a.dataDir, "export.json")

	// 导出所有数据库数据
	memories, _ := a.memory.GetMemories("")
	goals, _ := a.plan.GetAllGoals()
	settings, _ := a.settings.GetAll()
	observations, _ := a.memory.GetObservations()
	highlights, _ := a.GetHighlights()
	reflections, _ := a.GetReflections()

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
	// 清空数据库表
	tables := []string{"memories", "conversations", "messages", "goals", "milestones", "check_ins", "observations", "highlights", "reflections"}
	for _, table := range tables {
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
	value, err := a.settings.Get("onboarding_completed")
	if err != nil {
		return false, err
	}
	return value == "true", nil
}

// CompleteOnboarding 完成引导流程
func (a *App) CompleteOnboarding(userName string) error {
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
	return a.automationEngine.GetService().GetTasks(taskType)
}

// GetAutomationTask 获取单个自动化任务
func (a *App) GetAutomationTask(id int) (*models.AutomationTask, error) {
	return a.automationEngine.GetService().GetTask(id)
}

// CreateAutomationTask 创建自动化任务
func (a *App) CreateAutomationTask(name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool) (int, error) {
	task := &models.AutomationTask{
		Name:            name,
		Description:     description,
		TaskType:        taskType,
		Config:          config,
		ScheduleType:    scheduleType,
		ScheduleConfig:  scheduleConfig,
		Enabled:         enabled,
		MaxRetries:      2,
		RetryIntervalSec: 30,
	}
	id, err := a.automationEngine.GetService().CreateTask(task)
	if err != nil {
		return 0, err
	}
	// 如果启用，加入调度
	if enabled {
		a.automationEngine.ScheduleTask(id)
	}
	return id, nil
}

// UpdateAutomationTask 更新自动化任务
func (a *App) UpdateAutomationTask(id int, name, description, taskType, config, scheduleType, scheduleConfig string, enabled bool) error {
	task := &models.AutomationTask{
		ID:              id,
		Name:            name,
		Description:     description,
		TaskType:        taskType,
		Config:          config,
		ScheduleType:    scheduleType,
		ScheduleConfig:  scheduleConfig,
		Enabled:         enabled,
		MaxRetries:      2,
		RetryIntervalSec: 30,
	}
	err := a.automationEngine.GetService().UpdateTask(task)
	if err != nil {
		return err
	}
	// 重新调度
	return a.automationEngine.ScheduleTask(id)
}

// DeleteAutomationTask 删除自动化任务
func (a *App) DeleteAutomationTask(id int) error {
	a.automationEngine.UnscheduleTask(id)
	return a.automationEngine.GetService().DeleteTask(id)
}

// ToggleAutomationTask 启用/禁用自动化任务
func (a *App) ToggleAutomationTask(id int, enabled bool) error {
	err := a.automationEngine.GetService().ToggleTask(id, enabled)
	if err != nil {
		return err
	}
	if enabled {
		return a.automationEngine.ScheduleTask(id)
	}
	a.automationEngine.UnscheduleTask(id)
	return nil
}

// RunAutomationTaskNow 立即执行自动化任务
func (a *App) RunAutomationTaskNow(id int) (*models.AutomationExecution, error) {
	exec := a.automationEngine.ExecuteTask(id)
	if exec == nil {
		return nil, fmt.Errorf("执行失败")
	}
	return exec, nil
}

// GetAutomationExecutions 获取执行记录
func (a *App) GetAutomationExecutions(taskID int) ([]models.AutomationExecution, error) {
	return a.automationEngine.GetService().GetExecutions(taskID)
}

// GetAutomationSteps 获取workflow步骤
func (a *App) GetAutomationSteps(taskID int) ([]models.AutomationStep, error) {
	return a.automationEngine.GetService().GetSteps(taskID)
}

// SaveAutomationSteps 保存workflow步骤
func (a *App) SaveAutomationSteps(taskID int, stepsJSON string) error {
	err := a.automationEngine.GetService().SaveStepsJSON(taskID, stepsJSON)
	if err != nil {
		return err
	}
	return a.automationEngine.ScheduleTask(taskID)
}

// GetStepExecutions 获取步骤执行详情
func (a *App) GetStepExecutions(executionID int) ([]models.StepExecution, error) {
	return a.automationEngine.GetService().GetStepExecutions(executionID)
}

// GetAutomationDependencies 获取任务依赖关系
func (a *App) GetAutomationDependencies(taskID int) ([]models.AutomationDependency, error) {
	return a.automationEngine.GetService().GetDependencies(taskID)
}

// GetAutomationDependents 获取依赖于指定任务的任务
func (a *App) GetAutomationDependents(taskID int) ([]models.AutomationDependency, error) {
	return a.automationEngine.GetService().GetDependents(taskID)
}

// AddAutomationDependency 添加任务依赖
func (a *App) AddAutomationDependency(taskID, dependsOnID int, condition string) error {
	return a.automationEngine.GetService().AddDependency(taskID, dependsOnID, condition)
}

// RemoveAutomationDependency 删除任务依赖
func (a *App) RemoveAutomationDependency(id int) error {
	return a.automationEngine.GetService().RemoveDependency(id)
}

// GetTaskConfigSchema 获取任务类型配置表单Schema
func (a *App) GetTaskConfigSchema(taskType string) ([]models.ConfigField, error) {
	return a.automationEngine.GetRegistry().GetSchema(taskType), nil
}

// GetAllTaskSchemas 获取所有任务类型的Schema
func (a *App) GetAllTaskSchemas() map[string][]models.ConfigField {
	return a.automationEngine.GetRegistry().GetAllSchemas()
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
		"content":  result.Content,
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
