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
// 执行控制：默认按 steps 数组顺序执行；当某步设置了 NextOnSuccess / NextOnFailure
// 且下标合法时，改为按其指定的 step_index 跳转，支持条件分支工作流。
func (r *Runner) Run(ctx context.Context, plan Plan, vars map[string]string, callback ProgressCallback) *Result {
	startTime := time.Now()

	if vars == nil {
		vars = make(map[string]string)
	}

	result := &Result{
		Steps:     make([]StepResult, 0, len(plan.Steps)),
		Variables: vars,
	}

	// 记录已访问过的 step_index 用于环路检测（防止 NextOnSuccess
	// 形成自环导致无限循环；命中后立即终止并标记为失败）。
	visited := make(map[int]int)
	const maxLoopGuard = 1024
	stepLoops := 0

	i := 0
	for i < len(plan.Steps) {
		select {
		case <-ctx.Done():
			result.Success = false
			result.Error = "执行被取消"
			result.Duration = time.Since(startTime).Milliseconds()
			return result
		default:
		}

		stepLoops++
		if stepLoops > maxLoopGuard {
			result.Success = false
			result.Error = fmt.Sprintf("工作流执行超过最大步数 (%d)，疑似循环", maxLoopGuard)
			result.Duration = time.Since(startTime).Milliseconds()
			return result
		}

		step := plan.Steps[i]
		stepStart := time.Now()

		// 环路检测
		if visited[i] >= 3 {
			result.Success = false
			result.Error = fmt.Sprintf("检测到步骤 %d 多次执行（已访问 %d 次），停止以避免死循环", i, visited[i])
			result.Duration = time.Since(startTime).Milliseconds()
			return result
		}
		visited[i]++

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
				// 条件不满足仍按线性顺序继续
				i++
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
			r.emit(callback, ProgressEvent{
				Type:      "error",
				StepIndex: i,
				StepName:  step.AgentName,
				Content:   sr.Error,
				Done:      false,
				Duration:  sr.Duration,
			})
			// 失败时尝试跳到 NextOnFailure，否则按 OnError 决策
			if next := r.nextIndex(plan, i, false); next != i+1 {
				i = next
				continue
			}
			if step.OnError != "skip" {
				result.Success = false
				result.Error = sr.Error
				result.Duration = time.Since(startTime).Milliseconds()
				return result
			}
			i++
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
			if next := r.nextIndex(plan, i, false); next != i+1 {
				i = next
				continue
			}
			if step.OnError != "skip" {
				result.Success = false
				result.Error = sr.Error
				result.Duration = time.Since(startTime).Milliseconds()
				return result
			}
			i++
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

		// 成功跳转到 NextOnSuccess，否则按数组顺序继续
		i = r.nextIndex(plan, i, true)
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

// nextIndex 计算下一跳的下标。
// success=true → 用 NextOnSuccess；否则 NextOnFailure。
// 字段值 <=0 或越界 → 退化为 "i+1"（线性继续）；
// 故意把 "0 当无效" 与 step 数组下标冲突 0 区分开
// （用 1-based 配置成本高、用户体验差，故约定 0/负数 = 走默认）。
func (r *Runner) nextIndex(plan Plan, i int, success bool) int {
	if i < 0 || i >= len(plan.Steps) {
		return i + 1
	}
	step := plan.Steps[i]
	target := step.NextOnSuccess
	if !success {
		target = step.NextOnFailure
	}
	if target <= 0 || target >= len(plan.Steps) {
		return i + 1
	}
	return target
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
