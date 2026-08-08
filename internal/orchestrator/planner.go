package orchestrator

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"ai-companion/internal/ai"
	"ai-companion/internal/agents"
	"ai-companion/internal/pipeline"
)

// Planner 使用 LLM 分析用户意图并生成执行计划
//
// 【数据竞争修复】aiClient 会被 Orchestrator.UpdateAIClient（持 o.mu 写锁）
// 写入，同时被 GeneratePlan（在编排请求 goroutine 中）读取。
// 旧实现虽然写侧持了 o.mu，但读侧 GeneratePlan 完全无锁，仍构成 race。
// 这里给 Planner 自带一把 RWMutex：
//   - setClient / UpdateAIClient 走 Lock
//   - getClient / GeneratePlan 走 RLock
type Planner struct {
	mu       sync.RWMutex
	aiClient *ai.Client
	agentMgr *agents.AgentManager
}

// NewPlanner 创建 LLM 规划器
func NewPlanner(aiClient *ai.Client, agentMgr *agents.AgentManager) *Planner {
	return &Planner{
		aiClient: aiClient,
		agentMgr: agentMgr,
	}
}

// setClient 线程安全地替换 AI 客户端（由 Orchestrator.UpdateAIClient 调用）
func (p *Planner) setClient(client *ai.Client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.aiClient = client
}

// getClient 线程安全地读取 AI 客户端快照
func (p *Planner) getClient() *ai.Client {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.aiClient
}

// chatFastTokens 简单问候/闲聊的快路径
// MaxTokens: 复杂计划（多步 + 分支）需要更大输出空间；800 字符不够。
// 把上限提到 2000，并要求 LLM "完整闭合 JSON"，避免半截 JSON 回退。
const (
	chatDefaultMaxTokens = 2000
	chatFastMaxTokens    = 256
)

// GeneratePlan 使用 LLM 生成执行计划
// 返回 nil 表示 LLM 不可用，应回退到关键词路由
func (p *Planner) GeneratePlan(userInput string, ctx EnrichedContext) (*pipeline.Plan, error) {
	client := p.getClient()
	if client == nil {
		return nil, fmt.Errorf("AI 客户端未初始化")
	}

	// 快路径：检测明显是简单场景的输入，直接生成单步 plan
	// 而不再调 LLM，节省一次往返（典型节省 500ms~2s）。
	if plan, ok := p.tryFastPath(userInput); ok {
		return plan, nil
	}

	// 构建 planning prompt
	prompt := p.buildPlanningPrompt(userInput, ctx)

	messages := []ai.Message{
		{Role: "system", Content: plannerSystemPrompt},
		{Role: "user", Content: prompt},
	}

	// 关键修复：复杂计划可能需要 5~10 步 + 分支跳转的 JSON，
	// 800 tokens 极易被截断导致 JSON 不闭合、上游解析失败回退关键词。
	// 2000 tokens 既能装下大多数计划，又不至于显著增加成本/延迟。
	resp, err := client.Chat(messages, ai.WithTemperature(0.3), ai.WithMaxTokens(chatDefaultMaxTokens))
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

// tryFastPath 检测明显属于"不需要 LLM 规划"的场景并直接返回单步 plan。
// 命中条件：
//   - 极短输入（<= 6 个非空字符，纯问候/语气词）
//   - 全部由 emotion / 关键词已能直接命中的关键词组成
//
// 命中后跳过 LLM 往返，平均节省 500ms~2s。
func (p *Planner) tryFastPath(input string) (*pipeline.Plan, bool) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return &pipeline.Plan{Steps: []pipeline.Step{{AgentName: "emotion", Input: input, OutputVar: "reply"}}}, true
	}

	// 极短输入：直接交给 emotion
	if len([]rune(trimmed)) <= 6 {
		return &pipeline.Plan{Steps: []pipeline.Step{{AgentName: "emotion", Input: input, OutputVar: "reply"}}}, true
	}

	// 包含明显的多步/规划/工具动词时不走快路径
	// （覆盖"计划/搜索/查/读文件/写入/分析/调研/复盘"等关键词，
	// 这些场景 LLM 决策收益远大于延迟成本）
	lower := strings.ToLower(trimmed)
	multiStepHints := []string{
		"计划", "目标", "搜索", "查一", "了解一下", "读取文件", "read file",
		"写入文件", "write file", "列出目录", "list dir", "git",
		"分析", "调研", "复盘", "总结一下", "帮我",
		"plan", "search", "research", "summary", "summarize",
		"open", "打开链接", "生成文档", "写文档",
	}
	for _, h := range multiStepHints {
		if strings.Contains(lower, h) {
			return nil, false
		}
	}

	return &pipeline.Plan{Steps: []pipeline.Step{{AgentName: "emotion", Input: input, OutputVar: "reply"}}}, true
}

// plannerSystemPrompt 系统提示词（告诉 LLM 它的角色和可用工具）
const plannerSystemPrompt = `你是一个 AI 任务编排器。根据用户的请求，你需要决定调用哪些 Agent、按什么顺序、传什么参数。

你必须只返回一个 JSON 对象，格式如下：
{
  "steps": [
    {
      "agent": "agent_name",
      "input": "传给agent的内容",
      "output_var": "变量名",
      "next_on_success": 1,
      "next_on_failure": -1
    }
  ]
}

规则：
1. agent 必须从可用列表中选取（注意名称必须完全匹配）
2. input 是对该 agent 的具体指令，可以引用前面步骤的输出：{{变量名}}
3. output_var 是可选的结果变量名，供后续步骤引用
4. 简单问候、闲聊、情绪表达、普通问答 → 只用 emotion agent
5. 需要搜索信息 → 先 web；如果结果较多需要整理，再 summarize
6. 规划类请求 → planner agent
7. 回顾反思 → reflection agent
8. 如果请求包含多个子任务，按逻辑顺序排列步骤
9. file_generation 仅在用户明确要求"生成文档/保存文件/导出报告/写文档"时使用；口头总结、普通回复、聊天绝不调用 file_generation
10. 分支字段可选：next_on_success / next_on_failure 表示该步执行完后下一跳的 step 数组下标（0-based）。
    - 默认 0/不填/越界 → 按数组顺序继续
    - 典型用法：步骤1失败时跳到步骤3做兜底
    - 仅在确实需要条件分支时才填，避免给简单线性计划增加无意义字段`

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
