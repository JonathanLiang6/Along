package orchestrator

import (
	"fmt"
	"strings"

	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// ContextProvider 自动注入 Memory、Plan、对话历史到 Agent 上下文
type ContextProvider struct {
	memoryService       *services.MemoryService
	conversationService *services.ConversationService
	planService         *services.PlanService
}

// NewContextProvider 创建上下文提供器
func NewContextProvider(
	memory *services.MemoryService,
	conversation *services.ConversationService,
	plan *services.PlanService,
) *ContextProvider {
	return &ContextProvider{
		memoryService:       memory,
		conversationService: conversation,
		planService:         plan,
	}
}

// EnrichedContext 富化的上下文信息
type EnrichedContext struct {
	PlansSummary       string // 当前计划的摘要
	MemoriesSummary    string // 关键记忆的摘要
	HistorySummary     string // 最近对话的摘要
	SettingsSummary    string // 用户偏好设置
	HasRelevantContext bool   // 是否有相关内容
}

// Enrich 从各数据源收集上下文信息
func (cp *ContextProvider) Enrich(userInput string) EnrichedContext {
	ctx := EnrichedContext{}

	// 1. 收集计划上下文
	ctx.PlansSummary = cp.collectPlanContext(userInput)

	// 2. 收集记忆上下文
	ctx.MemoriesSummary = cp.collectMemoryContext(userInput)

	// 3. 收集对话历史
	ctx.HistorySummary = cp.collectHistoryContext()

	ctx.HasRelevantContext = ctx.PlansSummary != "" || ctx.MemoriesSummary != ""

	return ctx
}

// collectPlanContext 收集与用户输入相关的计划
func (cp *ContextProvider) collectPlanContext(userInput string) string {
	if cp.planService == nil {
		return ""
	}

	goals, err := cp.planService.GetAllGoals()
	if err != nil || len(goals) == 0 {
		return ""
	}

	var active []models.Goal
	for _, g := range goals {
		if g.Status == "active" {
			active = append(active, g)
			if len(active) >= 5 {
				break
			}
		}
	}

	if len(active) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, g := range active {
		sb.WriteString(fmt.Sprintf("- [%s] %s (进度:%d%%)\n", g.Type, g.Title, g.Progress))
		if g.CurrentFocus != "" {
			sb.WriteString(fmt.Sprintf("  当前关注: %s\n", g.CurrentFocus))
		}
	}
	return sb.String()
}

// collectMemoryContext 收集与用户输入相关的记忆
func (cp *ContextProvider) collectMemoryContext(userInput string) string {
	if cp.memoryService == nil {
		return ""
	}

	// 先尝试精确搜索
	mems, err := cp.memoryService.SearchMemories(userInput)
	if err != nil || len(mems) == 0 {
		// 没有精确匹配，取置信度最高的记忆
		mems, err = cp.memoryService.GetKeyMemories(5)
		if err != nil || len(mems) == 0 {
			return ""
		}
	}

	var sb strings.Builder
	for _, m := range mems {
		sb.WriteString(fmt.Sprintf("- [%s] %s (置信度:%.0f%%)\n", m.Type, m.Content, m.Confidence*100))
		if sb.Len() > 500 {
			break
		}
	}
	return sb.String()
}

// collectHistoryContext 收集最近对话历史
func (cp *ContextProvider) collectHistoryContext() string {
	if cp.conversationService == nil {
		return ""
	}

	msgs, err := cp.conversationService.GetRecentMessages(5)
	if err != nil || len(msgs) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, m := range msgs {
		role := "用户"
		if m.Role == "assistant" || m.Role == "companion" {
			role = "Along"
		}
		content := m.Content
		if len([]rune(content)) > 80 {
			content = string([]rune(content)[:80]) + "..."
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, content))
	}
	return sb.String()
}
