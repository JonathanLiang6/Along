package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"strings"
)

// EmotionAgent 情感陪伴 Agent
type EmotionAgent struct {
	BaseAgent
}

func (ea *EmotionAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "emotion", Description: "日常对话、情绪支持、陪伴聊天", InputDesc: "用户的聊天内容", OutputDesc: "关怀性回复"},
	}
}

// NewEmotionAgent 创建情感陪伴 Agent
func NewEmotionAgent(aiClient *ai.Client) *EmotionAgent {
	return &EmotionAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "emotion",
			desc:     "情感支持：日常对话、状态识别、情绪支持",
		},
	}
}

// Match 计算匹配度
func (ea *EmotionAgent) Match(ctx AgentContext) float64 {
	// 情感陪伴是默认 Agent，匹配度适中
	keywords := []string{
		"开心", "难过", "伤心", "生气", "焦虑", "累", "无聊",
		"想你", "聊聊", "说话", "聊天", "陪伴", "孤单",
		"今天过得", "怎么样", "好吗", "在吗",
	}
	score := KeywordMatch(ctx.Content, keywords)
	// 基础匹配度 0.4，作为兜底
	if score < 0.4 {
		return 0.4
	}
	return score
}

// Process 同步处理
func (ea *EmotionAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if ea.aiClient == nil {
		return &AgentResult{
			Content: "我在。有什么需要聊的吗？",
			Emotion: "专业",
		}, nil
	}

	systemPrompt := ai.BuildSystemPrompt("emotion", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	resp, err := ea.aiClient.Chat(messages, ai.WithTemperature(0.8))
	if err != nil {
		return &AgentResult{
			Content: "我在。有什么需要聊的吗？",
			Emotion: "专业",
		}, nil
	}

	emotion := detectEmotion(ctx.Content)
	return &AgentResult{
		Content:      resp,
		Emotion:      emotion,
		ShouldRecord: true,
	}, nil
}

// ProcessStream 流式处理
func (ea *EmotionAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if ea.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我在。有什么需要聊的吗？", Done: true})
		}
		return nil
	}

	systemPrompt := ai.BuildSystemPrompt("emotion", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	return ea.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.8))
}

// 构建消息列表
func buildMessages(ctx AgentContext, systemPrompt string) []ai.Message {
	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}

	// 添加历史消息（最近 10 条）
	history := ctx.History
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	messages = append(messages, history...)

	// 添加当前消息
	messages = append(messages, ai.Message{Role: "user", Content: ctx.Content})

	return messages
}

// 构建记忆上下文
func buildMemoryContext(ctx AgentContext) string {
	if len(ctx.Memory) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n以下是关于用户的一些记忆（仅供参考，不要直接复述）：\n")
	for i, m := range ctx.Memory {
		if i >= 15 {
			break
		}
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", m.Type, m.Content))
	}
	return sb.String()
}

// 简单的情绪检测
func detectEmotion(content string) string {
	lower := strings.ToLower(content)

	happyWords := []string{"开心", "高兴", "快乐", "哈哈", "棒", "好", "喜欢", "爱", "成功", "完成"}
	sadWords := []string{"难过", "伤心", "哭", "失落", "沮丧", "不开心", "痛苦", "难受"}
	angryWords := []string{"生气", "愤怒", "气死", "讨厌", "烦", "恼火"}
	anxiousWords := []string{"焦虑", "紧张", "担心", "害怕", "压力", "慌", "不安"}
	tiredWords := []string{"累", "疲惫", "困", "想睡觉", "没精神", "乏力"}

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
	for _, w := range anxiousWords {
		if strings.Contains(lower, w) {
			return "支持"
		}
	}
	for _, w := range tiredWords {
		if strings.Contains(lower, w) {
			return "专业"
		}
	}

	return "专业"
}
