package orchestrator

import (
	"context"
	"fmt"
	"log"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/pipeline"
	"ai-companion/internal/services"
)

// Orchestrator 主 Agent 编排器
// 职责: 接收用户输入 → 理解意图 → 生成计划 → 调度子Agent执行 → 返回结果
type Orchestrator struct {
	planner      *Planner
	keywordRouter *KeywordRouter
	pipeline     *pipeline.Runner
	contextProvider *ContextProvider
	aiClient     *ai.Client
	agentMgr     *agents.AgentManager
}

// New 创建编排器
func New(
	aiClient *ai.Client,
	agentMgr *agents.AgentManager,
	memorySvc *services.MemoryService,
	conversationSvc *services.ConversationService,
	planSvc *services.PlanService,
) *Orchestrator {
	return &Orchestrator{
		planner:         NewPlanner(aiClient, agentMgr),
		keywordRouter:   NewKeywordRouter(agentMgr),
		pipeline:        pipeline.NewRunner(agentMgr),
		contextProvider: NewContextProvider(memorySvc, conversationSvc, planSvc),
		aiClient:        aiClient,
		agentMgr:        agentMgr,
	}
}

// Process 处理用户输入（同步模式）
// 返回最终回复内容和执行计划（用于记录）
func (o *Orchestrator) Process(userInput string) (*ProcessResult, error) {
	startTime := time.Now()

	// 1. 注入上下文
	enrichedCtx := o.contextProvider.Enrich(userInput)

	// 2. 生成执行计划（LLM 优先，关键词兜底）
	var plan *pipeline.Plan
	var planSource string

	if o.aiClient != nil {
		p, err := o.planner.GeneratePlan(userInput, enrichedCtx)
		if err != nil {
			log.Printf("[Orchestrator] LLM 规划失败，回退到关键词路由: %v", err)
			plan = o.keywordRouter.Route(userInput)
			planSource = "keyword_fallback"
		} else {
			plan = p
			planSource = "llm"
		}
	} else {
		plan = o.keywordRouter.Route(userInput)
		planSource = "keyword"
	}

	if plan == nil || len(plan.Steps) == 0 {
		return &ProcessResult{
			Content:    "抱歉，我暂时无法处理这个请求。",
			PlanSource: planSource,
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 3. 执行计划
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	result := o.pipeline.Run(ctx, *plan, nil, nil)

	// 4. 构建返回结果
	content := result.Content
	if content == "" && len(result.Steps) > 0 {
		// 取最后一步的内容
		lastStep := result.Steps[len(result.Steps)-1]
		content = lastStep.Content
	}
	if content == "" {
		content = "任务已完成。"
	}

	return &ProcessResult{
		Content:    content,
		Plan:       plan,
		PlanSource: planSource,
		Result:     result,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// ProcessStream 处理用户输入（流式模式）
// 通过回调函数实时推送每个步骤的执行进度
func (o *Orchestrator) ProcessStream(userInput string, callback pipeline.ProgressCallback) (*ProcessResult, error) {
	startTime := time.Now()

	// 1. 注入上下文
	enrichedCtx := o.contextProvider.Enrich(userInput)

	// 2. 生成执行计划
	var plan *pipeline.Plan
	var planSource string

	if o.aiClient != nil {
		p, err := o.planner.GeneratePlan(userInput, enrichedCtx)
		if err != nil {
			log.Printf("[Orchestrator] LLM 规划失败，回退到关键词路由: %v", err)
			plan = o.keywordRouter.Route(userInput)
			planSource = "keyword_fallback"
		} else {
			plan = p
			planSource = "llm"
		}
	} else {
		plan = o.keywordRouter.Route(userInput)
		planSource = "keyword"
	}

	if plan == nil || len(plan.Steps) == 0 {
		content := "抱歉，我暂时无法处理这个请求。"
		if callback != nil {
			callback(pipeline.ProgressEvent{Type: "plan_done", Content: content, Done: true})
		}
		return &ProcessResult{
			Content:    content,
			PlanSource: planSource,
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 3. 流式执行计划
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// 流式包装：把 step 进度转发给 callback，同时把 agent 输出作为 chunk 推送
	streamCallback := func(event pipeline.ProgressEvent) {
		if callback != nil {
			callback(event)
		}
	}

	result := o.pipeline.Run(ctx, *plan, nil, streamCallback)

	// 4. 构建返回结果
	content := result.Content
	if content == "" && len(result.Steps) > 0 {
		content = result.Steps[len(result.Steps)-1].Content
	}

	return &ProcessResult{
		Content:    content,
		Plan:       plan,
		PlanSource: planSource,
		Result:     result,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// GenerateWorkflow 根据用户描述自动生成工作流步骤（供前端 WorkflowEditor 使用）
func (o *Orchestrator) GenerateWorkflow(description string) (*pipeline.Plan, error) {
	ctx := o.contextProvider.Enrich(description)

	if o.aiClient == nil {
		return nil, fmt.Errorf("AI 客户端未初始化")
	}

	return o.planner.GeneratePlan(description, ctx)
}

// UpdateAIClient 更新 AI 客户端
func (o *Orchestrator) UpdateAIClient(client *ai.Client) {
	o.aiClient = client
	o.planner.aiClient = client
}

// GetPipeline 获取流水线执行器
func (o *Orchestrator) GetPipeline() *pipeline.Runner {
	return o.pipeline
}

// ProcessResult 编排处理结果
type ProcessResult struct {
	Content    string           `json:"content"`     // 最终回复内容
	Plan       *pipeline.Plan   `json:"plan"`        // 生成的执行计划
	PlanSource string           `json:"plan_source"` // "llm" / "keyword" / "keyword_fallback"
	Result     *pipeline.Result `json:"result"`      // 流水线执行结果
	Duration   int64            `json:"duration_ms"` // 总耗时（毫秒）
}
