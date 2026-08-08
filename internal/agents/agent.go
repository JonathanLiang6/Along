package agents

import (
	"ai-companion/internal/ai"
	"strings"
	"sync"
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
//
// 【数据竞争修复】aiClient 字段会被两条 goroutine 并发访问：
//   - 写侧：CompanionCore.UpdateAIClient → AgentManager.UpdateAIClients
//     → 各 Agent.UpdateAIClient（用户切换 provider / API Key 时触发）
//   - 读侧：各 Agent.Process / ProcessStream / CreatePlan 等（流式对话时触发）
//
// 旧实现完全无锁，go test -race 会直接告警，且在极端时序下可能读到半更新
// 的指针值导致 panic。这里加 sync.RWMutex 保护：
//   - UpdateAIClient 用 Lock（写）
//   - GetAIClient 用 RLock（读），返回指针快照
// 所有子 Agent 的 Process / ProcessStream 必须改用 GetAIClient() 读取，
// 不能再直接访问 b.aiClient 字段。
type BaseAgent struct {
	mu       sync.RWMutex
	aiClient *ai.Client
	name     string
	desc     string
}

func (b *BaseAgent) Name() string                { return b.name }
func (b *BaseAgent) Description() string         { return b.desc }
func (b *BaseAgent) Capabilities() []Capability  { return nil }

// UpdateAIClient 线程安全地替换 AI 客户端
func (b *BaseAgent) UpdateAIClient(client *ai.Client) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.aiClient = client
}

// GetAIClient 线程安全地读取 AI 客户端快照
// 返回的是某一时刻的指针副本，调用方拿到后即可安全使用，
// 即使另一条 goroutine 随后调用 UpdateAIClient 也不影响本次调用。
func (b *BaseAgent) GetAIClient() *ai.Client {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.aiClient
}

// setAIClientInternal 仅用于构造函数（NewXxxAgent）内部初始化，
// 不暴露给外部，避免绕过锁直接写。
func (b *BaseAgent) setAIClientInternal(client *ai.Client) {
	b.aiClient = client
}

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
