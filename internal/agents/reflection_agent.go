package agents

import (
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ReflectionAgent 复盘分析 Agent
type ReflectionAgent struct {
	BaseAgent
	memoryService *services.MemoryService
	convService   *services.ConversationService
}

// NewReflectionAgent 创建复盘 Agent
func (ra *ReflectionAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "reflection", Description: "周期复盘、成长分析、关系回顾", InputDesc: "复盘周期(day/week/month)", OutputDesc: "结构化复盘报告"},
	}
}

func NewReflectionAgent(aiClient *ai.Client, memoryService *services.MemoryService, convService *services.ConversationService) *ReflectionAgent {
	return &ReflectionAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "reflection",
			desc:     "复盘分析：成长回顾、关系复盘、项目总结",
		},
		memoryService: memoryService,
		convService:   convService,
	}
}

// Match 计算匹配度
func (ra *ReflectionAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"复盘", "回顾", "总结", "这段时间", "最近", "成长",
		"我进步了", "反思", "检视", "这周", "这个月",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (ra *ReflectionAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if ra.aiClient == nil {
		return &AgentResult{
			Content: "好，我们来复盘。这段时间你做得不错的地方是坚持了下来，可以改进的地方是休息不够。整体来看进展是正向的。",
			Emotion: "认真",
		}, nil
	}

	chatCtx := ra.collectChatContext(30)
	memCtx := ra.collectMemoryContext()

	systemPrompt := fmt.Sprintf(`你现在处于复盘模式。用户想要回顾和总结最近的时间。

以下是近期对话记录：
%s

已记录的记忆：
%s

请生成一段复盘，包含：
1. 用户成长分析（进步了什么，哪些地方可以更好）
2. 关系成长分析（你和用户之间的变化）
3. 项目复盘（如果有项目相关的内容）
4. 观察和建议

以助手的口吻，客观而真诚。`, chatCtx, memCtx)

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: ctx.Content})

	resp, err := ra.aiClient.Chat(messages, ai.WithTemperature(0.7), ai.WithMaxTokens(1500))
	if err != nil {
		return &AgentResult{
			Content: "复盘需要整理一下信息。我们稍后再来梳理这段时间的情况，可以吗？",
			Emotion: "专业",
		}, nil
	}

	return &AgentResult{
		Content:      resp,
		Emotion:      "认真",
		ShouldRecord: true,
	}, nil
}

// ProcessStream 流式处理
func (ra *ReflectionAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if ra.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "好，我们来复盘。这段时间你做得不错的地方是坚持了下来，可以改进的地方是休息不够。整体来看进展是正向的。", Done: true})
		}
		return nil
	}

	chatCtx := ra.collectChatContext(30)
	memCtx := ra.collectMemoryContext()

	systemPrompt := fmt.Sprintf(`你现在处于复盘模式。用户想要回顾和总结最近的时间。

以下是近期对话记录：
%s

已记录的记忆：
%s

请生成一段复盘，包含：
1. 用户成长分析（进步了什么，哪些地方可以更好）
2. 关系成长分析（你和用户之间的变化）
3. 项目复盘（如果有项目相关的内容）
4. 观察和建议

以助手的口吻，客观而真诚。`, chatCtx, memCtx)

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: ctx.Content})

	return ra.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.7), ai.WithMaxTokens(1500))
}

// Generate 生成复盘报告
// period: day / week / month
func (ra *ReflectionAgent) Generate(period string) (*models.Reflection, error) {
	end := time.Now()
	var start time.Time
	switch period {
	case "day":
		start = end.AddDate(0, 0, -1)
	case "month":
		start = end.AddDate(0, -1, 0)
	default:
		start = end.AddDate(0, 0, -7)
		period = "week"
	}

	periodStart := start.Format("2006-01-02")
	periodEnd := end.Format("2006-01-02")

	if ra.aiClient == nil {
		return &models.Reflection{
			PeriodStart:          periodStart,
			PeriodEnd:            periodEnd,
			GrowthAnalysis:       "这段时间你坚持了记录和反思，这就是最大的成长。",
			RelationshipAnalysis: "我们之间的关系越来越默契。",
			ProjectReview:        "项目按计划推进。",
			Observations:         "继续保持。",
		}, nil
	}

	chatCtx := ra.collectChatContext(50)
	memCtx := ra.collectMemoryContext()

	prompt := fmt.Sprintf(`复盘周期: %s 至 %s (%s)

近期对话摘要:
%s

已记录的关键记忆:
%s

请生成结构化复盘，返回严格 JSON 格式（不要 markdown 代码块）:
{
  "growth_analysis": "用户成长分析（进步+待改进）",
  "relationship_analysis": "我们关系的变化",
  "project_review": "项目相关回顾",
  "observations": "总体观察与建议"
}`, periodStart, periodEnd, period, chatCtx, memCtx)

	resp, err := ra.aiClient.Chat([]ai.Message{
		{Role: "system", Content: "你是一个有温度的复盘助手。结构化输出 JSON。"},
		{Role: "user", Content: prompt},
	}, ai.WithTemperature(0.7), ai.WithMaxTokens(1500))

	if err != nil {
		return &models.Reflection{
			PeriodStart:          periodStart,
			PeriodEnd:            periodEnd,
			GrowthAnalysis:       "（LLM 暂时无法响应，请稍后重试）",
			RelationshipAnalysis: "",
			ProjectReview:        "",
			Observations:         "",
		}, nil
	}

	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var result struct {
		GrowthAnalysis       string `json:"growth_analysis"`
		RelationshipAnalysis string `json:"relationship_analysis"`
		ProjectReview        string `json:"project_review"`
		Observations         string `json:"observations"`
	}
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		result.GrowthAnalysis = resp
	}

	reflection := &models.Reflection{
		PeriodStart:          periodStart,
		PeriodEnd:            periodEnd,
		GrowthAnalysis:       result.GrowthAnalysis,
		RelationshipAnalysis: result.RelationshipAnalysis,
		ProjectReview:        result.ProjectReview,
		Observations:         result.Observations,
	}

	_ = ra.memoryService.SaveReflection(reflection)
	return reflection, nil
}

// 收集对话上下文
func (ra *ReflectionAgent) collectChatContext(limit int) string {
	if ra.convService == nil {
		return "(无对话记录)"
	}
	recentMsgs, err := ra.convService.GetRecentMessages(limit)
	if err != nil || len(recentMsgs) == 0 {
		return "(无对话记录)"
	}

	var sb strings.Builder
	for _, msg := range recentMsgs {
		role := "用户"
		if msg.Role == "assistant" || msg.Role == "companion" {
			role = "Along"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, msg.Content))
	}
	return sb.String()
}

// 收集记忆上下文
func (ra *ReflectionAgent) collectMemoryContext() string {
	if ra.memoryService == nil {
		return "(无记忆记录)"
	}
	memories, err := ra.memoryService.GetMemories("")
	if err != nil || len(memories) == 0 {
		return "(无记忆记录)"
	}

	var sb strings.Builder
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", m.Type, m.Content))
	}
	return sb.String()
}
