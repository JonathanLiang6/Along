package agents

import (
	"ai-companion/internal/ai"
	"ai-companion/internal/services"
	"fmt"
	"strings"
)

// MemoryAgent 记忆管理 Agent
type MemoryAgent struct {
	BaseAgent
	memoryService *services.MemoryService
}

// NewMemoryAgent 创建记忆 Agent
func (ma *MemoryAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "memory", Description: "记忆提取、分类存储、回忆查询", InputDesc: "查询关键词或需要记忆的内容", OutputDesc: "相关记忆列表"},
	}
}

func NewMemoryAgent(aiClient *ai.Client, memoryService *services.MemoryService) *MemoryAgent {
	return &MemoryAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "memory",
			desc:     "记忆管理：提取、存储、检索用户记忆",
		},
		memoryService: memoryService,
	}
}

// Match 计算匹配度
func (ma *MemoryAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"记得", "记住", "忘记了", "回忆", "之前", "上次", "我告诉过你",
		"我的生日", "我喜欢", "我不喜欢", "你还记得吗",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (ma *MemoryAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if ma.aiClient == nil {
		return &AgentResult{
			Content: "我会记录关于你的重要信息。有什么需要我记住的吗？",
			Emotion: "专业",
		}, nil
	}

	systemPrompt := ai.BuildSystemPrompt("memory", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	resp, err := ma.aiClient.Chat(messages, ai.WithTemperature(0.7))
	if err != nil {
		return &AgentResult{
			Content: "我会记录关于你的重要信息。有什么需要我记住的吗？",
			Emotion: "专业",
		}, nil
	}

	memUpdates := ma.extractMemories(ctx.Content)
	return &AgentResult{
		Content:      resp,
		Emotion:      "专业",
		ShouldRecord: true,
		MemoryUpdate: memUpdates,
	}, nil
}

// ProcessStream 流式处理
func (ma *MemoryAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if ma.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我会记录关于你的重要信息。有什么需要我记住的吗？", Done: true})
		}
		return nil
	}

	systemPrompt := ai.BuildSystemPrompt("memory", buildMemoryContext(ctx))
	messages := buildMessages(ctx, systemPrompt)

	return ma.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.7))
}

// extractMemories 从对话中提取记忆
func (ma *MemoryAgent) extractMemories(content string) []MemoryUpdate {
	if ma.aiClient == nil {
		return nil
	}

	lower := strings.ToLower(content)
	var updates []MemoryUpdate

	// L5 日常喜好
	if strings.Contains(lower, "我喜欢") || strings.Contains(lower, "我爱") || strings.Contains(lower, "我爱吃") {
		updates = append(updates, MemoryUpdate{
			Type:       "L5",
			Content:    content,
			Confidence: 0.8,
			Source:     "auto_extract",
		})
	}

	// L1 个人画像
	if strings.Contains(lower, "我叫") || strings.Contains(lower, "我的名字") || strings.Contains(lower, "我今年") {
		updates = append(updates, MemoryUpdate{
			Type:       "L1",
			Content:    content,
			Confidence: 0.9,
			Source:     "auto_extract",
		})
	}

	// L2 情感关系
	if strings.Contains(lower, "我的朋友") || strings.Contains(lower, "我家人") || strings.Contains(lower, "我对象") ||
		strings.Contains(lower, "我老婆") || strings.Contains(lower, "我老公") || strings.Contains(lower, "我妈") ||
		strings.Contains(lower, "我爸") {
		updates = append(updates, MemoryUpdate{
			Type:       "L2",
			Content:    content,
			Confidence: 0.75,
			Source:     "auto_extract",
		})
	}

	// L3 关键事件
	if strings.Contains(lower, "今天我") && (strings.Contains(lower, "第一次") ||
		strings.Contains(lower, "终于") || strings.Contains(lower, "重要")) {
		updates = append(updates, MemoryUpdate{
			Type:       "L3",
			Content:    content,
			Confidence: 0.7,
			Source:     "auto_extract",
		})
	}

	return updates
}

// Handle 处理记忆查询（兼容旧接口）
func (ma *MemoryAgent) Handle(content string) (string, error) {
	ctx := AgentContext{Content: content}
	result, err := ma.Process(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// ProcessMemoryChange 处理记忆变更（兼容旧接口）
func (ma *MemoryAgent) ProcessMemoryChange(content string) (*AgentResponse, error) {
	ctx := AgentContext{Content: content}
	result, err := ma.Process(ctx)
	if err != nil {
		return nil, err
	}
	return &AgentResponse{
		Content: result.Content,
		Emotion: result.Emotion,
		Data:    result.Data,
	}, nil
}

// SummarizeMemories 摘要记忆
func (ma *MemoryAgent) SummarizeMemories(memType string, count int) (string, error) {
	if ma.memoryService == nil {
		return "", fmt.Errorf("memory service not available")
	}

	mems, err := ma.memoryService.GetMemories(memType)
	if err != nil || len(mems) == 0 {
		return "还没有相关记忆。", nil
	}

	if count > len(mems) {
		count = len(mems)
	}

	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(fmt.Sprintf("- [%s] %s\n", mems[i].Type, mems[i].Content))
	}

	if ma.aiClient == nil {
		return sb.String(), nil
	}

	prompt := fmt.Sprintf("请将以下记忆用简短的话总结一下，温暖自然：\n\n%s", sb.String())
	resp, err := ma.aiClient.Chat([]ai.Message{
		{Role: "system", Content: "你是一个善于总结的助手。"},
		{Role: "user", Content: prompt},
	}, ai.WithTemperature(0.7), ai.WithMaxTokens(300))

	if err != nil {
		return sb.String(), nil
	}

	return resp, nil
}
