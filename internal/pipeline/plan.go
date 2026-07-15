package pipeline

// Step 流水线中的一个执行步骤
type Step struct {
	AgentName string `json:"agent"`            // 要调用的 Agent 名称
	Input     string `json:"input"`            // 输入内容，支持 {{var}} 变量替换
	OutputVar string `json:"output_var"`       // 将结果存入此变量名
	Condition string `json:"condition,omitempty"` // 条件表达式，不满足则跳过
	OnError   string `json:"on_error,omitempty"` // "skip" 或 "abort"，默认 abort
}

// Plan 由 Orchestrator 生成或用户手动编排的执行计划
type Plan struct {
	Steps []Step `json:"steps"`
}

// StepResult 单个步骤的执行结果
type StepResult struct {
	Index     int    `json:"index"`
	AgentName string `json:"agent"`
	Success   bool   `json:"success"`
	Content   string `json:"content"`
	Error     string `json:"error,omitempty"`
	Duration  int64  `json:"duration_ms"`
}

// Result 流水线整体执行结果
type Result struct {
	Success   bool              `json:"success"`
	Steps     []StepResult      `json:"steps"`
	Variables map[string]string `json:"variables"` // 所有步骤的输出变量
	Content   string            `json:"content"`   // 最后一步的完整输出
	Error     string            `json:"error,omitempty"`
	Duration  int64             `json:"duration_ms"`
}
