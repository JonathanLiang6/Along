package pipeline

import (
	"context"
	"fmt"
	"strings"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
)

// Runner 流水线执行器
type Runner struct {
	agentMgr *agents.AgentManager
}

// NewRunner 创建流水线执行器
func NewRunner(agentMgr *agents.AgentManager) *Runner {
	return &Runner{agentMgr: agentMgr}
}

// ProgressCallback 步骤进度回调
type ProgressCallback func(event ProgressEvent)

// ProgressEvent 执行进度事件
type ProgressEvent struct {
	Type      string `json:"type"` // "step_start", "step_done", "chunk", "plan_done", "error"
	StepIndex int    `json:"step_index,omitempty"`
	StepName  string `json:"step_name,omitempty"`
	Content   string `json:"content,omitempty"`
	Done      bool   `json:"done"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms,omitempty"`
}

// Run 按顺序执行计划中的所有步骤
// vars: 初始变量（可为 nil）
// callback: 进度回调（可为 nil）
func (r *Runner) Run(ctx context.Context, plan Plan, vars map[string]string, callback ProgressCallback) *Result {
	startTime := time.Now()

	if vars == nil {
		vars = make(map[string]string)
	}

	result := &Result{
		Steps:     make([]StepResult, 0, len(plan.Steps)),
		Variables: vars,
	}

	for i, step := range plan.Steps {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = "执行被取消"
			result.Duration = time.Since(startTime).Milliseconds()
			return result
		default:
		}

		stepStart := time.Now()

		// 条件检查
		if step.Condition != "" {
			if !EvaluateCondition(step.Condition, vars) {
				r.emit(callback, ProgressEvent{
					Type:      "step_done",
					StepIndex: i,
					StepName:  step.AgentName,
					Content:   "skipped: condition not met",
					Done:      false,
				})
				result.Steps = append(result.Steps, StepResult{
					Index:     i,
					AgentName: step.AgentName,
					Success:   true,
					Content:   "(skipped)",
				})
				continue
			}
		}

		// 解析变量
		input := resolveVars(step.Input, vars)

		r.emit(callback, ProgressEvent{
			Type:      "step_start",
			StepIndex: i,
			StepName:  step.AgentName,
			Content:   fmt.Sprintf("正在执行: %s", step.AgentName),
			Done:      false,
		})

		// 查找并调用 Agent
		agent, ok := r.agentMgr.GetAgent(step.AgentName)
		if !ok {
			sr := StepResult{
				Index:     i,
				AgentName: step.AgentName,
				Success:   false,
				Error:     fmt.Sprintf("找不到Agent: %s", step.AgentName),
				Duration:  time.Since(stepStart).Milliseconds(),
			}
			result.Steps = append(result.Steps, sr)
			if step.OnError != "skip" {
				result.Success = false
				result.Error = sr.Error
				result.Duration = time.Since(startTime).Milliseconds()
				return result
			}
			continue
		}

		// 构建 AgentContext 并调用
		agentCtx := agents.AgentContext{
			Content: input,
			History: []ai.Message{},
			Extra:   map[string]interface{}{},
		}
		response, err := agent.Process(agentCtx)
		duration := time.Since(stepStart).Milliseconds()

		if err != nil {
			sr := StepResult{
				Index:     i,
				AgentName: step.AgentName,
				Success:   false,
				Error:     err.Error(),
				Duration:  duration,
			}
			result.Steps = append(result.Steps, sr)
			r.emit(callback, ProgressEvent{
				Type:      "error",
				StepIndex: i,
				StepName:  step.AgentName,
				Content:   err.Error(),
				Done:      false,
				Duration:  duration,
			})
			if step.OnError != "skip" {
				result.Success = false
				result.Error = sr.Error
				result.Duration = time.Since(startTime).Milliseconds()
				return result
			}
			continue
		}

		content := response.Content

		// 保存输出变量
		if step.OutputVar != "" {
			vars[step.OutputVar] = truncateContent(content)
		}

		sr := StepResult{
			Index:     i,
			AgentName: step.AgentName,
			Success:   true,
			Content:   content,
			Duration:  duration,
		}
		result.Steps = append(result.Steps, sr)
		result.Content = content // 最后一步的内容

		r.emit(callback, ProgressEvent{
			Type:      "step_done",
			StepIndex: i,
			StepName:  step.AgentName,
			Content:   content,
			Done:      false,
			Duration:  duration,
		})
	}

	result.Success = true
	result.Duration = time.Since(startTime).Milliseconds()

	r.emit(callback, ProgressEvent{
		Type:    "plan_done",
		Content: "执行完成",
		Done:    true,
	})

	return result
}

// RunWithStream 流式执行计划（支持 LLM 流式输出）
func (r *Runner) RunWithStream(ctx context.Context, plan Plan, vars map[string]string, callback ProgressCallback) *Result {
	// 目前与 Run 相同，后续可扩展为支持 ProcessStream
	return r.Run(ctx, plan, vars, callback)
}

// emit 发送进度回调
func (r *Runner) emit(callback ProgressCallback, event ProgressEvent) {
	if callback != nil {
		callback(event)
	}
}

// truncateContent 截断内容用于变量存储（变量名只保存摘要）
func truncateContent(s string) string {
	s = strings.TrimSpace(s)
	// 仅取前 2000 字符作为变量值
	runes := []rune(s)
	if len(runes) > 2000 {
		return string(runes[:2000]) + "\n...(已截断)"
	}
	return s
}
