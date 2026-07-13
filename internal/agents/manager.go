package agents

import (
	"ai-companion/internal/ai"
	"sort"
	"sync"
)

// AgentManager Agent 管理器 - 负责注册、路由、调度
type AgentManager struct {
	mu     sync.RWMutex
	agents map[string]Agent
	routes []AgentRoute
}

// AgentRoute Agent 路由规则
type AgentRoute struct {
	AgentName string
	Priority  int
	Keywords  []string
}

// NewAgentManager 创建 Agent 管理器
func NewAgentManager() *AgentManager {
	return &AgentManager{
		agents: make(map[string]Agent),
		routes: make([]AgentRoute, 0),
	}
}

// Register 注册一个 Agent
func (am *AgentManager) Register(agent Agent) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.agents[agent.Name()] = agent
}

// RegisterRoute 注册路由规则
func (am *AgentManager) RegisterRoute(route AgentRoute) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.routes = append(am.routes, route)
	sort.Slice(am.routes, func(i, j int) bool {
		return am.routes[i].Priority > am.routes[j].Priority
	})
}

// GetAgent 根据名称获取 Agent
func (am *AgentManager) GetAgent(name string) (Agent, bool) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	agent, ok := am.agents[name]
	return agent, ok
}

// ListAgents 列出所有已注册的 Agent
func (am *AgentManager) ListAgents() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()
	names := make([]string, 0, len(am.agents))
	for name := range am.agents {
		names = append(names, name)
	}
	return names
}

// RouteResult 路由结果
type RouteResult struct {
	AgentName  string
	Confidence float64
	Agent      Agent
}

// Route 根据上下文路由到最合适的 Agent
func (am *AgentManager) Route(ctx AgentContext) RouteResult {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var results []RouteResult

	// 首先使用路由规则快速匹配
	for _, route := range am.routes {
		agent, ok := am.agents[route.AgentName]
		if !ok {
			continue
		}
		confidence := KeywordMatch(ctx.Content, route.Keywords)
		if confidence > 0 {
			results = append(results, RouteResult{
				AgentName:  route.AgentName,
				Confidence: confidence,
				Agent:      agent,
			})
		}
	}

	// 然后让每个 Agent 自己计算匹配度
	for name, agent := range am.agents {
		// 跳过已经在路由结果中的
		found := false
		for _, r := range results {
			if r.AgentName == name {
				found = true
				break
			}
		}
		if found {
			continue
		}

		confidence := agent.Match(ctx)
		if confidence > 0.1 {
			results = append(results, RouteResult{
				AgentName:  name,
				Confidence: confidence,
				Agent:      agent,
			})
		}
	}

	// 按置信度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Confidence > results[j].Confidence
	})

	// 返回置信度最高的，默认返回 emotion
	if len(results) > 0 && results[0].Confidence > 0.15 {
		return results[0]
	}

	// 默认使用情感陪伴 Agent
	if agent, ok := am.agents["emotion"]; ok {
		return RouteResult{
			AgentName:  "emotion",
			Confidence: 0.5,
			Agent:      agent,
		}
	}

	// 兜底：返回第一个可用的
	for name, agent := range am.agents {
		return RouteResult{
			AgentName:  name,
			Confidence: 0.3,
			Agent:      agent,
		}
	}

	return RouteResult{}
}

// Process 处理消息（同步）
func (am *AgentManager) Process(ctx AgentContext) (*AgentResult, error) {
	route := am.Route(ctx)
	if route.Agent == nil {
		return &AgentResult{
			Content: "我在，有什么需要帮忙的吗？",
			Emotion: "专业",
		}, nil
	}

	ctx.Intent = AgentIntent{
		Name:       route.AgentName,
		Confidence: route.Confidence,
	}

	return route.Agent.Process(ctx)
}

// ProcessStream 处理消息（流式）
func (am *AgentManager) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	route := am.Route(ctx)
	if route.Agent == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我在，有什么需要帮忙的吗？", Done: true})
		}
		return nil
	}

	ctx.Intent = AgentIntent{
		Name:       route.AgentName,
		Confidence: route.Confidence,
	}

	return route.Agent.ProcessStream(ctx, callback)
}

// UpdateAIClients 更新所有 Agent 的 AI 客户端
func (am *AgentManager) UpdateAIClients(client *ai.Client) {
	am.mu.RLock()
	defer am.mu.RUnlock()
	for _, agent := range am.agents {
		agent.UpdateAIClient(client)
	}
}
