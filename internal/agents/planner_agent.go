package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"strings"
)

// PlannerAgent 计划与目标管理 Agent
type PlannerAgent struct {
	BaseAgent
}

// NewPlannerAgent 创建计划 Agent
func NewPlannerAgent(aiClient *ai.Client) *PlannerAgent {
	return &PlannerAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "planner",
			desc:     "计划管理：目标设定、里程碑规划、进度追踪",
		},
	}
}

// Match 计算匹配度
func (pa *PlannerAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"计划", "目标", "规划", "里程碑", "任务", "项目", "todo", "待办",
		"学习计划", "工作计划", "安排", "进度", "完成", "怎么做",
		"开始", "启动", "制定", "安排", "分解",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (pa *PlannerAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if pa.aiClient == nil {
		return &AgentResult{
			Content: "我们一起来规划吧。告诉我你想做什么，我来帮你拆分成可以一步步完成的小目标。",
			Emotion: "认真",
		}, nil
	}

	systemPrompt := ai.BuildSystemPrompt("planner", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	resp, err := pa.aiClient.Chat(messages, ai.WithTemperature(0.7))
	if err != nil {
		return &AgentResult{
			Content: "我们一起来规划吧。告诉我你想做什么，我来帮你拆分成可以一步步完成的小目标。",
			Emotion: "认真",
		}, nil
	}

	return &AgentResult{
		Content:      resp,
		Emotion:      "认真",
		ShouldRecord: true,
	}, nil
}

// ProcessStream 流式处理
func (pa *PlannerAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if pa.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我们一起来规划吧。告诉我你想做什么，我来帮你拆分成可以一步步完成的小目标。", Done: true})
		}
		return nil
	}

	systemPrompt := ai.BuildSystemPrompt("planner", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	return pa.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.7))
}

// HandlePlanMessage 处理计划相关消息（保留兼容接口）
func (pa *PlannerAgent) HandlePlanMessage(content string, history []ai.Message) (string, error) {
	ctx := AgentContext{
		Content: content,
		History: history,
	}
	result, err := pa.Process(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// CreatePlan 创建计划建议
func (pa *PlannerAgent) CreatePlan(title, description, planType string) (string, error) {
	if pa.aiClient == nil {
		return fmt.Sprintf("好的，我们来做「%s」这个%s计划。先从简单的第一步开始吧。", title, planType), nil
	}

	prompt := fmt.Sprintf(`用户想要创建一个新计划：
- 标题：%s
- 描述：%s
- 类型：%s

请给出：
1. 热情的鼓励和肯定
2. 3-5个初始里程碑建议（可执行的小步骤）
3. 一点温馨提醒

以助手的口吻，温暖但专业。`, title, description, planType)

	resp, err := pa.aiClient.Chat([]ai.Message{
		{Role: "system", Content: ai.BuildSystemPrompt("planner", "")},
		{Role: "user", Content: prompt},
	}, ai.WithTemperature(0.7), ai.WithMaxTokens(800))

	if err != nil {
		return fmt.Sprintf("好的，我们来做「%s」这个%s计划。先从简单的第一步开始吧。", title, planType), nil
	}

	return resp, nil
}

// MilestoneComment 里程碑完成评论
func (pa *PlannerAgent) MilestoneComment(milestoneTitle string, goalTitle string) string {
	if pa.aiClient == nil {
		return "又完成了一个里程碑，做得好！继续加油。"
	}

	prompt := fmt.Sprintf(`用户完成了一个里程碑：
- 计划：%s
- 里程碑：%s

请用一句话（不超过30字）表示祝贺和鼓励，温暖真诚。`, goalTitle, milestoneTitle)

	resp, err := pa.aiClient.Chat([]ai.Message{
		{Role: "system", Content: "你是温暖的助手，给用户鼓励。"},
		{Role: "user", Content: prompt},
	}, ai.WithTemperature(0.8), ai.WithMaxTokens(100))

	if err != nil {
		return "又完成了一个里程碑，做得好！继续加油。"
	}

	resp = strings.TrimSpace(resp)
	if len(resp) == 0 {
		return "又完成了一个里程碑，做得好！继续加油。"
	}
	return resp
}
