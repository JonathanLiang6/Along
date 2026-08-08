package core

import (
	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/orchestrator"
	"ai-companion/internal/pipeline"
	"ai-companion/internal/services"
	"fmt"
	"strings"
	"sync"
	"time"
)

func modelsToAIMessages(msgs []models.Message) []ai.Message {
	result := make([]ai.Message, len(msgs))
	for i, m := range msgs {
		role := m.Role
		if role == "companion" {
			role = "assistant"
		}
		result[i] = ai.Message{
			Role:    role,
			Content: m.Content,
		}
	}
	return result
}

type CompanionCore struct {
	mu sync.RWMutex

	aiClient            *ai.Client
	agentManager        *agents.AgentManager
	orchestrator        *orchestrator.Orchestrator
	memoryService       *services.MemoryService
	conversationService *services.ConversationService
	planService         *services.PlanService
	automationService   *services.AutomationService
	taskExecutor        TaskExecutor

	emotionAgent        *agents.EmotionAgent
	plannerAgent        *agents.PlannerAgent
	memoryAgent         *agents.MemoryAgent
	researchAgent       *agents.ResearchAgent
	reflectionAgent     *agents.ReflectionAgent
	toolAgent           *agents.ToolAgent
	webAgent            *agents.WebAgent
	summarizeAgent      *agents.SummarizeAgent
	fileGenerationAgent *agents.FileGenerationAgent
	techAnalysisAgent   *agents.TechAnalysisAgent
}

type TaskExecutor interface {
	ExecuteTask(taskID int) *models.AutomationExecution
}

func NewCompanionCore(
	aiClient *ai.Client,
	memoryService *services.MemoryService,
	conversationService *services.ConversationService,
	planService *services.PlanService,
) *CompanionCore {
	cc := &CompanionCore{
		aiClient:            aiClient,
		memoryService:       memoryService,
		conversationService: conversationService,
		planService:         planService,
		agentManager:        agents.NewAgentManager(),
	}

	cc.emotionAgent = agents.NewEmotionAgent(aiClient)
	cc.plannerAgent = agents.NewPlannerAgent(aiClient)
	cc.memoryAgent = agents.NewMemoryAgent(aiClient, memoryService)
	cc.reflectionAgent = agents.NewReflectionAgent(aiClient, memoryService, conversationService)
	cc.toolAgent = agents.NewToolAgent(aiClient)
	cc.webAgent = agents.NewWebAgent(aiClient)
	cc.summarizeAgent = agents.NewSummarizeAgent(aiClient, cc.webAgent)
	cc.fileGenerationAgent = agents.NewFileGenerationAgent(aiClient)
	cc.techAnalysisAgent = agents.NewTechAnalysisAgent(aiClient, cc.webAgent)
	cc.researchAgent = agents.NewResearchAgent(aiClient, cc.webAgent)

	cc.agentManager.Register(cc.emotionAgent)
	cc.agentManager.Register(cc.plannerAgent)
	cc.agentManager.Register(cc.memoryAgent)
	cc.agentManager.Register(cc.researchAgent)
	cc.agentManager.Register(cc.reflectionAgent)
	cc.agentManager.Register(cc.toolAgent)
	cc.agentManager.Register(cc.webAgent)
	cc.agentManager.Register(cc.summarizeAgent)
	cc.agentManager.Register(cc.fileGenerationAgent)
	cc.agentManager.Register(cc.techAnalysisAgent)

	cc.agentManager.RegisterMutexGroup("search", []string{"web", "tech_analysis", "research"})

	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "tech_analysis",
		Priority:  92,
		Keywords:  []string{"什么是", "解释", "分析", "原理", "机制", "架构", "技术", "算法", "模型", "框架", "系统", "工作原理", "Loop", "Agentic", "RAG", "大模型", "LLM", "Transformer", "多模态", "扩散模型", "强化学习", "微调", "推理"},
	})

	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "planner",
		Priority:  100,
		Keywords:  []string{"计划", "目标", "里程碑", "任务", "项目", "todo", "待办", "学习计划", "工作计划"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "research",
		Priority:  90,
		Keywords:  []string{"深度调研", "专题研究", "文献综述", "全面了解", "深入分析", "系统性研究"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "reflection",
		Priority:  80,
		Keywords:  []string{"复盘", "回顾", "总结", "这段时间", "成长", "反思", "这周", "这个月"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "memory",
		Priority:  70,
		Keywords:  []string{"记得", "记住", "忘记了", "回忆", "之前", "上次", "我告诉过你"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "emotion",
		Priority:  10,
		Keywords:  []string{"开心", "难过", "伤心", "生气", "焦虑", "累", "无聊", "想你", "聊聊", "陪伴"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "tool",
		Priority:  85,
		Keywords:  []string{"读取文件", "read file", "写入文件", "write file", "列出目录", "list dir", "git状态", "git status", "git日志", "git log", "打开链接", "open browser"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "web",
		Priority:  95,
		Keywords:  []string{"搜索", "查一下", "了解一下", "新闻", "最新", "网页", "网站", "链接", "下载", "天气", "汇率", "bing", "必应"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "summarize",
		Priority:  88,
		Keywords:  []string{"总结", "整理", "归纳", "梳理", "摘要", "提炼", "汇总", "概括", "整合", "分类整理"},
	})
	cc.agentManager.RegisterRoute(agents.AgentRoute{
		AgentName: "file_generation",
		Priority:  87,
		Keywords:  []string{"生成文档", "生成报告", "保存文档", "导出文档", "生成markdown", "生成md", "周报文档", "报告文档", "文档模板"},
	})

	// 创建 Orchestrator（LLM规划 + 关键词兜底）
	cc.orchestrator = orchestrator.New(aiClient, cc.agentManager, memoryService, conversationService, planService)

	return cc
}

func (cc *CompanionCore) UpdateAIClient(client *ai.Client) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.aiClient = client
	cc.agentManager.UpdateAIClients(client)
	if cc.orchestrator != nil {
		cc.orchestrator.UpdateAIClient(client)
	}
}

// GetOrchestrator 获取编排器
func (cc *CompanionCore) GetOrchestrator() *orchestrator.Orchestrator {
	return cc.orchestrator
}

// detectSlashCommand 检测斜杠命令，返回 (command, 参数)
func detectSlashCommand(content string) (string, string) {
	trimmed := strings.TrimSpace(content)
	if !strings.HasPrefix(trimmed, "/") {
		return "", ""
	}
	parts := strings.SplitN(trimmed[1:], " ", 2)
	cmd := strings.ToLower(strings.TrimSpace(parts[0]))
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	return cmd, arg
}

// buildContextFromConversation 从对话ID构建Agent上下文
func (cc *CompanionCore) buildContextFromConversation(conversationID int, content string) (agents.AgentContext, error) {
	historyMsgs, err := cc.conversationService.GetMessagesByConversationID(conversationID)
	if err != nil {
		historyMsgs = []models.Message{}
	}

	maxHistory := 8
	if len(historyMsgs) > maxHistory {
		historyMsgs = historyMsgs[len(historyMsgs)-maxHistory:]
	}
	aiHistory := modelsToAIMessages(historyMsgs)

	mems, _ := cc.memoryService.GetMemories("")
	var memoryItems []agents.MemoryItem
	for _, m := range mems {
		memoryItems = append(memoryItems, agents.MemoryItem{
			Type:    m.Type,
			Content: m.Content,
		})
	}

	relevantMemories := cc.filterRelevantMemories(memoryItems, content, 6)

	ctx := agents.AgentContext{
		Content:   content,
		History:   aiHistory,
		Memory:    relevantMemories,
		SessionID: fmt.Sprintf("conv_%d", conversationID),
	}
	return ctx, nil
}

// filterRelevantMemories 过滤与当前内容相关的记忆
// 当没有关键词命中时（relevant 为空）且总记忆超过 limit，
// 不能直接返回空切片，否则上游会完全丢失记忆上下文。
// 此时应回退到按出现顺序取前 limit 条，保证至少有"概览"级别的上下文。
func (cc *CompanionCore) filterRelevantMemories(mems []agents.MemoryItem, content string, limit int) []agents.MemoryItem {
	if len(mems) == 0 {
		return mems
	}

	lowerContent := strings.ToLower(content)
	var relevant []agents.MemoryItem

	for _, m := range mems {
		if strings.Contains(lowerContent, strings.ToLower(m.Content)) {
			relevant = append(relevant, m)
			if len(relevant) >= limit {
				break
			}
		}
	}

	// 无命中时回退到 top-N（按出现顺序），避免上下文完全丢失。
	if len(relevant) == 0 {
		n := min(len(mems), limit)
		out := make([]agents.MemoryItem, n)
		copy(out, mems[:n])
		return out
	}

	return relevant
}

// parseMessageTimestamp 解析数据库消息时间戳
// SQLite 默认 datetime('now') 输出格式为 "2006-01-02 15:04:05"，
// 但历史数据/前端写入可能使用 RFC3339（"2006-01-02T15:04:05Z07:00"）。
// 此处按"最常见→最严格"顺序尝试，失败时返回零值而不是让上游误判。
func parseMessageTimestamp(ts string) time.Time {
	if ts == "" {
		return time.Time{}
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.000",
		time.RFC3339,
		time.RFC3339Nano,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, ts); err == nil {
			return t
		}
	}
	return time.Time{}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProcessMessageStreamInConversation 在指定对话中流式处理消息
//
// 【黑屏修复 / 死锁修复】重要：不持长锁！
// 旧实现从函数开始到结束全程持有 cc.mu.RLock()，这意味着：
//   1) 一次普通对话的 LLM 流式响应可能耗时 10~120 秒
//   2) 在这 10~120 秒内，任何写锁调用（UpdateAIClient / SetAutomationService
//      / SetTaskExecutor 等）都会被永久阻塞
//   3) 如果写锁调用方正持有 resources 等待本函数返回 → 死锁 → Wails 主循环
//      事件无法派发 → 启动卡死 → 看起来"黑屏"
// 新实现采用"快照 → 释放锁 → 业务处理"模式：先把需要的字段快照出来，释放
// 锁后再做真正的 LLM / 持久化工作。这样任何时刻只持锁几毫秒。
func (cc *CompanionCore) ProcessMessageStreamInConversation(
	conversationID int,
	content string,
	onChunk func(chunk ai.StreamChunk),
) (string, string, error) {
	// ============ 阶段 1：在锁内完成"读字段 → 决策 → 派活" ============
	type slashPlan struct {
		kind         string // "automation" / "builtin" / ""
		effective    string
		taskID       int
		taskName     string
		hasExecutor  bool
		hasAutoSvc   bool
		replyPrefix  string
		conversation *services.ConversationService
		automation   *services.AutomationService
	}

	var plan slashPlan
	var builtinCmd, builtinArg string

	cc.mu.RLock()
	builtinCmd, builtinArg = detectSlashCommand(content)
	plan.hasAutoSvc = cc.automationService != nil
	plan.automation = cc.automationService
	plan.conversation = cc.conversationService
	plan.hasExecutor = cc.taskExecutor != nil
	cc.mu.RUnlock()

	// ============ 阶段 2：处理斜杠命令（同样不持锁） ============
	if builtinCmd != "" {
		// 1) 自动化任务斜杠命令
		if plan.hasAutoSvc {
			task, err := plan.automation.GetTaskBySlashCommand("/" + builtinCmd)
			if err == nil && task != nil && task.ID > 0 {
				plan.kind = "automation"
				plan.taskID = task.ID
				plan.taskName = task.Name
				plan.replyPrefix = "正在执行任务：" + plan.taskName + "，请稍候..."
			}
		}

		// 2) 内置命令（plan / review / memory）
		if plan.kind == "" {
			switch builtinCmd {
			case "plan":
				if builtinArg != "" {
					plan.effective = "帮我制定一个关于「" + builtinArg + "」的计划，分解成可执行的步骤。"
				} else {
					plan.effective = "帮我制定一个计划，分解成可执行的步骤。"
				}
				plan.kind = "builtin"
			case "review":
				period := "本周"
				if builtinArg != "" {
					period = builtinArg
				}
				plan.effective = "帮我回顾一下" + period + "的情况，做一个总结。"
				plan.kind = "builtin"
			case "memory":
				if builtinArg != "" {
					plan.effective = "帮我回忆一下关于「" + builtinArg + "」的事情。"
				} else {
					plan.effective = "请列出你记得的关于我的事情。"
				}
				plan.kind = "builtin"
			default:
			}
		}
	}

	effectiveContent := content
	if plan.kind == "builtin" {
		effectiveContent = plan.effective
	}

	// ============ 阶段 3：自动化任务的"占位 + 异步"派活 ============
	if plan.kind == "automation" {
		conv := plan.conversation
		if conv == nil {
			return "", "专业", fmt.Errorf("对话服务未初始化")
		}
		if _, saveErr := conv.SaveMessageToConversation(conversationID, "user", content, ""); saveErr != nil {
			return "", "专业", saveErr
		}
		conv.UpdateConversationTitleByFirstMessage(conversationID, content)

		reply := plan.replyPrefix
		if onChunk != nil {
			onChunk(ai.StreamChunk{Content: reply, Done: false})
		}

		if !plan.hasExecutor {
			if onChunk != nil {
				onChunk(ai.StreamChunk{Content: "\n\n任务已提交执行。", Done: true})
			}
			fullReply := reply + "\n\n任务已提交执行。"
			conv.SaveMessageToConversation(conversationID, "assistant", fullReply, "专业")
			return reply, "专业", nil
		}

		// 把任务执行放到独立 goroutine，避免主流程等 LLM 阻塞
		go func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("[ProcessMessageStreamInConversation] 任务执行 panic: %v\n", r)
				}
			}()
			// 关键修复：再次取一次 executor，避免与 SetTaskExecutor 写锁冲突
			cc.mu.RLock()
			exec := cc.taskExecutor
			cc.mu.RUnlock()
			if exec == nil {
				return
			}
			taskExec := exec.ExecuteTask(plan.taskID)
			resultMsg := ""
			if taskExec.Status == "success" {
				resultMsg = "\n\n✅ 任务执行完成！"
				if taskExec.ResultContent != "" {
					if len(taskExec.ResultContent) > 2000 {
						resultMsg += "\n\n" + taskExec.ResultContent[:2000] + "\n...（内容已截断）"
					} else {
						resultMsg += "\n\n" + taskExec.ResultContent
					}
				}
				if taskExec.ResultPath != "" {
					resultMsg += "\n\n📁 输出文件: " + taskExec.ResultPath
				}
			} else if taskExec.Status == "failed" {
				resultMsg = "\n\n❌ 任务执行失败: " + taskExec.ErrorMessage
			} else {
				resultMsg = "\n\n⏳ 任务已提交，正在执行中..."
			}
			if onChunk != nil {
				onChunk(ai.StreamChunk{Content: resultMsg, Done: true})
			}
			fullReply := reply + resultMsg
			conv.SaveMessageToConversation(conversationID, "assistant", fullReply, "专业")
		}()

		return reply, "专业", nil
	}

	// ============ 阶段 4：常规消息（不持锁，调用 Orchestrator） ============
	conv := plan.conversation
	if conv == nil {
		return "", "专业", fmt.Errorf("对话服务未初始化")
	}

	if _, saveErr := conv.SaveMessageToConversation(conversationID, "user", content, ""); saveErr != nil {
		return "", "专业", saveErr
	}
	conv.UpdateConversationTitleByFirstMessage(conversationID, content)

	var fullReply string
	finalEmotion := "专业"

	// 关键修复：不持 cc.mu 锁调用 Orchestrator。
	// Orchestrator 内部自己有 aiClient 锁，调用期间即便 SetAutomationService
	// 等写锁等待也不会再与本函数死锁。
	// 期间通过 aiClient 字段的短锁读取来保持与 UpdateAIClient 的安全。
	cc.mu.RLock()
	orch := cc.orchestrator
	cc.mu.RUnlock()
	if orch == nil {
		return "", "专业", fmt.Errorf("编排器未初始化")
	}

	procResult, err := orch.ProcessStream(effectiveContent, func(event pipeline.ProgressEvent) {
		if event.Type == "step_done" && !event.Done && event.Content != "" {
			if fullReply != "" {
				fullReply += "\n\n"
			}
			fullReply += event.Content
		}
		if onChunk != nil {
			onChunk(ai.StreamChunk{
				Content:      event.Content,
				Done:         event.Done,
				FinishReason: event.Type,
			})
		}
	})

	if err != nil {
		fallback := "抱歉，刚才处理出了点问题，能再说一遍吗？"
		if onChunk != nil {
			onChunk(ai.StreamChunk{Content: fallback, Done: true})
		}
		conv.SaveMessageToConversation(conversationID, "assistant", fallback, "专业")
		return fallback, "专业", err
	}

	if fullReply == "" {
		fullReply = procResult.Content
	}
	finalEmotion = cc.detectEmotionSimple(effectiveContent)
	conv.SaveMessageToConversation(conversationID, "assistant", fullReply, finalEmotion)

	return fullReply, finalEmotion, nil
}

func (cc *CompanionCore) ProcessMessage(content string) (string, string, error) {
	// 关键修复：不持长锁调用 Orchestrator。
	// 之前 defer cc.mu.RUnlock() 会让整个 LLM 同步调用期间持有读锁，
	// 与 UpdateAIClient 等写锁形成潜在死锁路径。
	cc.mu.RLock()
	orch := cc.orchestrator
	conv := cc.conversationService
	cc.mu.RUnlock()

	if orch == nil {
		return "", "专业", fmt.Errorf("编排器未初始化")
	}

	procResult, err := orch.Process(content)
	if err != nil {
		return "", "专业", err
	}

	if conv != nil {
		conv.SaveMessage("user", content, "")
		conv.SaveMessage("assistant", procResult.Content, "")
	}

	return procResult.Content, "专业", nil
}

func (cc *CompanionCore) ProcessMessageStream(
	content string,
	onChunk func(chunk ai.StreamChunk),
) (string, string, error) {
	// 关键修复：不持长锁读 conversationService。
	// 之前 defer cc.mu.RUnlock() 让 GetOrCreateTodayConversation 期间
	// 持读锁，与 OnChange("api_provider") 写锁形成潜在死锁路径。
	cc.mu.RLock()
	conv := cc.conversationService
	cc.mu.RUnlock()
	if conv == nil {
		return "", "专业", fmt.Errorf("对话服务未初始化")
	}
	convID, err := conv.GetOrCreateTodayConversation()
	if err != nil {
		return "", "专业", err
	}
	return cc.ProcessMessageStreamInConversation(convID, content, onChunk)
}

func (cc *CompanionCore) detectEmotionSimple(content string) string {
	lower := strings.ToLower(content)
	happyWords := []string{"开心", "高兴", "快乐", "哈哈", "棒", "喜欢", "爱", "成功"}
	sadWords := []string{"难过", "伤心", "哭", "失落", "沮丧", "不开心"}
	angryWords := []string{"生气", "愤怒", "气死", "讨厌", "烦"}

	for _, w := range happyWords {
		if strings.Contains(lower, w) {
			return "开心"
		}
	}
	for _, w := range sadWords {
		if strings.Contains(lower, w) {
			return "关注"
		}
	}
	for _, w := range angryWords {
		if strings.Contains(lower, w) {
			return "支持"
		}
	}
	return "专业"
}

func (cc *CompanionCore) GenerateReflection(period string) (*models.Reflection, error) {
	return cc.reflectionAgent.Generate(period)
}

func (cc *CompanionCore) CreatePlanNote(title, description, planType string) string {
	note, err := cc.plannerAgent.CreatePlan(title, description, planType)
	if err != nil {
		return fmt.Sprintf("好的，我们来做「%s」这个计划。加油！", title)
	}
	return note
}

func (cc *CompanionCore) MilestoneComment(milestoneTitle, goalTitle string) string {
	return cc.plannerAgent.MilestoneComment(milestoneTitle, goalTitle)
}

func (cc *CompanionCore) GetAgentManager() *agents.AgentManager {
	return cc.agentManager
}

func (cc *CompanionCore) GetEmotionAgent() *agents.EmotionAgent {
	return cc.emotionAgent
}

func (cc *CompanionCore) GetPlannerAgent() *agents.PlannerAgent {
	return cc.plannerAgent
}

func (cc *CompanionCore) GetMemoryAgent() *agents.MemoryAgent {
	return cc.memoryAgent
}

func (cc *CompanionCore) GetResearchAgent() *agents.ResearchAgent {
	return cc.researchAgent
}

func (cc *CompanionCore) GetReflectionAgent() *agents.ReflectionAgent {
	return cc.reflectionAgent
}

func (cc *CompanionCore) GetToolAgent() *agents.ToolAgent {
	return cc.toolAgent
}

func (cc *CompanionCore) GetWebAgent() *agents.WebAgent {
	return cc.webAgent
}

func (cc *CompanionCore) SetAutomationService(s *services.AutomationService) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.automationService = s
}

func (cc *CompanionCore) GetAutomationService() *services.AutomationService {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.automationService
}

func (cc *CompanionCore) SetTaskExecutor(e TaskExecutor) {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	cc.taskExecutor = e
}

func (cc *CompanionCore) getTaskExecutor() (TaskExecutor, bool) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.taskExecutor, cc.taskExecutor != nil
}

func (cc *CompanionCore) GetSummarizeAgent() *agents.SummarizeAgent {
	return cc.summarizeAgent
}

func (cc *CompanionCore) GetFileGenerationAgent() *agents.FileGenerationAgent {
	return cc.fileGenerationAgent
}

func (cc *CompanionCore) GenerateProactiveContent() ([]models.Observation, error) {
	var observations []models.Observation

	// 关键修复：conversationService / memoryService 可能因 asyncInit 失败
	// 而为 nil。直接调用会 panic，让前端 GetProactiveContent 接口直接崩溃。
	// 这里改为快照读取并做 nil 检查；任一缺失时返回一个兜底问候观察。
	cc.mu.RLock()
	conv := cc.conversationService
	memSvc := cc.memoryService
	cc.mu.RUnlock()

	today := time.Now().Format("2006-01-02")
	todayProactive := 0
	weekProactive := 0
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	if conv != nil {
		msgs, _ := conv.GetRecentMessages(50)
		for _, m := range msgs {
			if m.Role == "assistant" {
				// 历史库使用 SQLite datetime('now')（"2006-01-02 15:04:05"），
				// 不能用 RFC3339 解析，否则时间永远为零值、计数永远为 0。
				msgTime := parseMessageTimestamp(m.Timestamp)
				if msgTime.IsZero() {
					continue
				}
				if msgTime.Format("2006-01-02") == today {
					if strings.HasPrefix(m.Content, "【提醒】") || strings.HasPrefix(m.Content, "【观察】") {
						todayProactive++
					}
				}
				if msgTime.After(weekAgo) {
					if strings.HasPrefix(m.Content, "【提醒】") || strings.HasPrefix(m.Content, "【观察】") {
						weekProactive++
					}
				}
			}
		}
	}

	var mems []models.Memory
	if memSvc != nil {
		var err error
		mems, err = memSvc.GetMemories("fact")
		if err != nil {
			mems = nil
		}
	}
	if len(mems) > 0 && todayProactive < 1 && weekProactive < 3 {
		for _, m := range mems {
			if strings.Contains(m.Content, "生日") || strings.Contains(m.Content, "纪念日") {
				observations = append(observations, models.Observation{
					Type:    "reminder",
					Content: "【提醒】" + m.Content + "，请注意安排。",
				})
				break
			}
		}
	}

	if len(observations) == 0 && todayProactive < 1 {
		observations = append(observations, models.Observation{
			Type:    "greeting",
			Content: "今天有什么需要处理的任务吗？",
		})
	}

	return observations, nil
}
