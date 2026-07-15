package agents

import (
	"ai-companion/internal/ai"
	"strings"
)

// AgentIntent 表示识别出的用户意图
type AgentIntent struct {
	Name       string
	Confidence float64
	Keywords   []string
	Metadata   map[string]string
}

// AgentContext Agent 执行上下文
type AgentContext struct {
	UserID    string
	Content   string
	History   []ai.Message
	Memory    []MemoryItem
	Intent    AgentIntent
	SessionID string
	RequestID string
	Extra     map[string]interface{}
}

// MemoryItem 简化的记忆项
type MemoryItem struct {
	Type    string
	Content string
}

// AgentResult Agent 执行结果
type AgentResult struct {
	Content      string
	Emotion      string
	Action       string
	Data         interface{}
	ShouldRecord bool
	MemoryUpdate []MemoryUpdate
}

// MemoryUpdate 记忆更新
type MemoryUpdate struct {
	Type       string
	Content    string
	Confidence float64
	Source     string
}

// Capability Agent 能力声明（供 Orchestrator 的 LLM Planner 了解每个 Agent 能做什么）
type Capability struct {
	Name        string `json:"name"`        // 能力名称，如 "web_search"
	Description string `json:"description"` // 一句话描述
	InputDesc   string `json:"input_desc"`  // 输入说明，如 "搜索关键词"
	OutputDesc  string `json:"output_desc"` // 输出说明，如 "搜索结果文本"
}

// StreamCallback 流式回调函数
type StreamCallback func(chunk ai.StreamChunk)

// Agent 所有 Agent 必须实现的统一接口
type Agent interface {
	Name() string
	Description() string
	Match(ctx AgentContext) float64
	Process(ctx AgentContext) (*AgentResult, error)
	ProcessStream(ctx AgentContext, callback StreamCallback) error
	UpdateAIClient(client *ai.Client)
	Capabilities() []Capability // 声明 Agent 的能力列表，供 Orchestrator 使用
}

// AgentResponse 旧版响应类型（向后兼容）
type AgentResponse struct {
	Content string
	Emotion string
	Data    interface{}
}

// BaseAgent 提供 Agent 的基础实现
type BaseAgent struct {
	aiClient *ai.Client
	name     string
	desc     string
}

func (b *BaseAgent) Name() string                     { return b.name }
func (b *BaseAgent) Description() string              { return b.desc }
func (b *BaseAgent) UpdateAIClient(client *ai.Client) { b.aiClient = client }
func (b *BaseAgent) Capabilities() []Capability       { return nil }

// KeywordMatch 基于关键词的基础匹配度计算
func KeywordMatch(content string, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}
	lower := strings.ToLower(content)
	matched := 0
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			matched++
		}
	}
	return float64(matched) / float64(len(keywords))
}
