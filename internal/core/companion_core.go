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

	if len(relevant) == 0 && len(mems) <= limit {
		return mems[:min(len(mems), limit)]
	}

	return relevant
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ProcessMessageStreamInConversation 在指定对话中流式处理消息
func (cc *CompanionCore) ProcessMessageStreamInConversation(
	conversationID int,
	content string,
	onChunk func(chunk ai.StreamChunk),
) (string, string, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	cmd, arg := detectSlashCommand(content)
	effectiveContent := content

	if cmd != "" {
		if cc.automationService != nil {
			task, err := cc.automationService.GetTaskBySlashCommand("/" + cmd)
			if err == nil && task != nil && task.ID > 0 {
				_, _ = cc.conversationService.SaveMessageToConversation(conversationID, "user", content, "")
				cc.conversationService.UpdateConversationTitleByFirstMessage(conversationID, content)

				reply := "正在执行任务：" + task.Name + "，请稍候..."
				if onChunk != nil {
					onChunk(ai.StreamChunk{Content: reply, Done: false})
				}

				if taskExecutor, ok := cc.getTaskExecutor(); ok {
					go func() {
						exec := taskExecutor.ExecuteTask(task.ID)
						resultMsg := ""
						if exec.Status == "success" {
							resultMsg = "\n\n✅ 任务执行完成！"
							if exec.ResultContent != "" {
								if len(exec.ResultContent) > 2000 {
									resultMsg += "\n\n" + exec.ResultContent[:2000] + "\n...（内容已截断）"
								} else {
									resultMsg += "\n\n" + exec.ResultContent
								}
							}
							if exec.ResultPath != "" {
								resultMsg += "\n\n📁 输出文件: " + exec.ResultPath
							}
						} else if exec.Status == "failed" {
							resultMsg = "\n\n❌ 任务执行失败: " + exec.ErrorMessage
						} else {
							resultMsg = "\n\n⏳ 任务已提交，正在执行中..."
						}
						if onChunk != nil {
							onChunk(ai.StreamChunk{Content: resultMsg, Done: true})
						}
						fullReply := reply + resultMsg
						cc.conversationService.SaveMessageToConversation(conversationID, "assistant", fullReply, "专业")
					}()
				} else {
					if onChunk != nil {
						onChunk(ai.StreamChunk{Content: "\n\n任务已提交执行。", Done: true})
					}
					fullReply := reply + "\n\n任务已提交执行。"
					cc.conversationService.SaveMessageToConversation(conversationID, "assistant", fullReply, "专业")
				}

				return reply, "专业", nil
			}
		}

		switch cmd {
		case "plan":
			if arg != "" {
				effectiveContent = "帮我制定一个关于「" + arg + "」的计划，分解成可执行的步骤。"
			} else {
				effectiveContent = "帮我制定一个计划，分解成可执行的步骤。"
			}
		case "review":
			period := "本周"
			if arg != "" {
				period = arg
			}
			effectiveContent = "帮我回顾一下" + period + "的情况，做一个总结。"
		case "memory":
			if arg != "" {
				effectiveContent = "帮我回忆一下关于「" + arg + "」的事情。"
			} else {
				effectiveContent = "请列出你记得的关于我的事情。"
			}
		default:
		}
	}

	if _, saveErr := cc.conversationService.SaveMessageToConversation(conversationID, "user", content, ""); saveErr != nil {
		return "", "专业", saveErr
	}

	cc.conversationService.UpdateConversationTitleByFirstMessage(conversationID, content)

	var fullReply string
	finalEmotion := "专业"

	// 使用 Orchestrator 流式处理
	procResult, err := cc.orchestrator.ProcessStream(effectiveContent, func(event pipeline.ProgressEvent) {
		if event.Type == "step_done" || event.Type == "plan_done" {
			if event.Content != "" && !event.Done {
				fullReply += event.Content
			}
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
		cc.conversationService.SaveMessageToConversation(conversationID, "assistant", fallback, "专业")
		return fallback, "专业", err
	}

	fullReply = procResult.Content
	finalEmotion = cc.detectEmotionSimple(effectiveContent)
	cc.conversationService.SaveMessageToConversation(conversationID, "assistant", fullReply, finalEmotion)

	return fullReply, finalEmotion, nil
}

func (cc *CompanionCore) ProcessMessage(content string) (string, string, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	// 使用 Orchestrator（LLM优先 + 关键词兜底，上下文由 Orchestrator 内部注入）
	procResult, err := cc.orchestrator.Process(content)
	if err != nil {
		return "", "专业", err
	}

	cc.conversationService.SaveMessage("user", content, "")
	cc.conversationService.SaveMessage("assistant", procResult.Content, "")

	return procResult.Content, "专业", nil
}

func (cc *CompanionCore) ProcessMessageStream(
	content string,
	onChunk func(chunk ai.StreamChunk),
) (string, string, error) {
	cc.mu.RLock()
	convID, err := cc.conversationService.GetOrCreateTodayConversation()
	cc.mu.RUnlock()
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

	today := time.Now().Format("2006-01-02")
	msgs, _ := cc.conversationService.GetRecentMessages(50)
	todayProactive := 0
	weekProactive := 0
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)

	for _, m := range msgs {
		if m.Role == "assistant" {
			msgTime, _ := time.Parse(time.RFC3339, m.Timestamp)
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

	mems, err := cc.memoryService.GetMemories("fact")
	if err == nil && len(mems) > 0 && todayProactive < 1 && weekProactive < 3 {
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
