package orchestrator

import (
	"ai-companion/internal/agents"
	"ai-companion/internal/pipeline"
)

// KeywordRouter LLM 不可用时的关键词匹配回退路由器
// 保留原 AgentManager.Route 的核心逻辑，但输出格式统一为 Plan
type KeywordRouter struct {
	agentMgr *agents.AgentManager
}

// NewKeywordRouter 创建关键词路由器
func NewKeywordRouter(agentMgr *agents.AgentManager) *KeywordRouter {
	return &KeywordRouter{agentMgr: agentMgr}
}

// Route 使用关键词匹配选择最佳 Agent，包装为单步 Plan
func (kr *KeywordRouter) Route(userInput string) *pipeline.Plan {
	ctx := agents.AgentContext{Content: userInput}

	// 使用原 AgentManager 的路由逻辑
	result := kr.agentMgr.Route(ctx)

	agentName := "emotion"
	if result.Agent != nil && result.AgentName != "" {
		agentName = result.AgentName
	}

	// 包装为单步计划
	return &pipeline.Plan{
		Steps: []pipeline.Step{
			{
				AgentName: agentName,
				Input:     userInput,
				OutputVar: "reply",
			},
		},
	}
}
