package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"

	"ai-companion/internal/ai"
	"ai-companion/internal/agents"
	"ai-companion/internal/pipeline"
)

// Planner 使用 LLM 分析用户意图并生成执行计划
type Planner struct {
	aiClient  *ai.Client
	agentMgr  *agents.AgentManager
}

// NewPlanner 创建 LLM 规划器
func NewPlanner(aiClient *ai.Client, agentMgr *agents.AgentManager) *Planner {
	return &Planner{
		aiClient: aiClient,
		agentMgr: agentMgr,
	}
}

// GeneratePlan 使用 LLM 生成执行计划
// 返回 nil 表示 LLM 不可用，应回退到关键词路由
func (p *Planner) GeneratePlan(userInput string, ctx EnrichedContext) (*pipeline.Plan, error) {
	if p.aiClient == nil {
		return nil, fmt.Errorf("AI 客户端未初始化")
	}

	// 构建 planning prompt
	prompt := p.buildPlanningPrompt(userInput, ctx)

	messages := []ai.Message{
		{Role: "system", Content: plannerSystemPrompt},
		{Role: "user", Content: prompt},
	}

	resp, err := p.aiClient.Chat(messages, ai.WithTemperature(0.3), ai.WithMaxTokens(800))
	if err != nil {
		return nil, fmt.Errorf("LLM 规划失败: %w", err)
	}

	// 解析 LLM 返回的 JSON plan
	plan, err := p.parsePlanResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("解析计划失败: %w", err)
	}

	return plan, nil
}

// plannerSystemPrompt 系统提示词（告诉 LLM 它的角色和可用工具）
const plannerSystemPrompt = `你是一个 AI 任务编排器。根据用户的请求，你需要决定调用哪些 Agent、按什么顺序、传什么参数。

你必须只返回一个 JSON 对象，格式如下：
{
  "steps": [
    {"agent": "agent_name", "input": "传给agent的内容", "output_var": "变量名"}
  ]
}

规则：
1. agent 必须从可用列表中选取（注意名称必须完全匹配）
2. input 是对该 agent 的具体指令，可以引用前面步骤的输出：{{变量名}}
3. output_var 是可选的结果变量名，供后续步骤引用
4. 简单问候或闲聊 → 只用 emotion agent
5. 需要搜索 → 先 web，再 summarize（如果需要整理），最后 file_generation（如果需要保存）
6. 规划类请求 → planner agent
7. 回顾反思 → reflection agent
8. 如果请求包含多个子任务，按逻辑顺序排列步骤`

// buildPlanningPrompt 构建规划提示词
func (p *Planner) buildPlanningPrompt(userInput string, ctx EnrichedContext) string {
	var sb strings.Builder

	sb.WriteString("## 可用 Agent 列表\n\n")

	// 列出所有已注册的 agent 及其能力描述
	for _, name := range p.agentMgr.ListAgents() {
		agent, ok := p.agentMgr.GetAgent(name)
		if !ok {
			continue
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", agent.Name(), agent.Description()))
	}

	sb.WriteString("\n## 用户请求\n\n")
	sb.WriteString(userInput)

	if ctx.HasRelevantContext {
		sb.WriteString("\n\n## 相关上下文\n")
		if ctx.PlansSummary != "" {
			sb.WriteString("\n当前计划:\n")
			sb.WriteString(ctx.PlansSummary)
		}
		if ctx.MemoriesSummary != "" {
			sb.WriteString("\n关键记忆:\n")
			sb.WriteString(ctx.MemoriesSummary)
		}
		if ctx.HistorySummary != "" {
			sb.WriteString("\n最近对话:\n")
			sb.WriteString(ctx.HistorySummary)
		}
	}

	sb.WriteString("\n## 请返回执行计划（JSON）\n")
	sb.WriteString("只返回 JSON，不要其他文字。")

	return sb.String()
}

// parsePlanResponse 解析 LLM 返回的计划 JSON
func (p *Planner) parsePlanResponse(resp string) (*pipeline.Plan, error) {
	resp = strings.TrimSpace(resp)

	// 清理 markdown 代码块
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	// 尝试解析为完整的 JSON 对象
	var plan pipeline.Plan
	if err := json.Unmarshal([]byte(resp), &plan); err != nil {
		// 兼容：尝试只解析 steps 数组
		var steps []pipeline.Step
		if err2 := json.Unmarshal([]byte(resp), &steps); err2 != nil {
			return nil, fmt.Errorf("无法解析 LLM 响应为 Plan: %w (原始: %s)", err, truncateStr(resp, 200))
		}
		plan.Steps = steps
	}

	return &plan, nil
}

// truncateStr 截断字符串
func truncateStr(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "..."
}
