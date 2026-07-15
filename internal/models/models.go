package models

// Message 消息模型
type Message struct {
	ID             int    `json:"id"`
	ConversationID int    `json:"conversation_id"`
	Role           string `json:"role"` // user / assistant
	Content        string `json:"content"`
	Emotion        string `json:"emotion,omitempty"`
	Timestamp      string `json:"timestamp"`
}

// Conversation 对话模型
type Conversation struct {
	ID         int    `json:"id"`
	Date       string `json:"date"`
	Title      string `json:"title"`
	AgentRoute string `json:"agent_route,omitempty"`
	CreatedAt  string `json:"created_at"`
}

// Memory 记忆模型
// PRD 5：5层记忆体系
//   L1 个人画像（如：名字、年龄、职业）
//   L2 情感关系（家人、朋友、伴侣）
//   L3 关键事件（重要日期、转折点）
//   L4 项目目标（正在做的事）
//   L5 日常喜好（口味、爱好、习惯）
type Memory struct {
	ID         int     `json:"id"`
	Type       string  `json:"type"` // L1/L2/L3/L4/L5
	Level      int     `json:"level"`
	Content    string  `json:"content"`
	Source     string  `json:"source,omitempty"`
	Confidence float64 `json:"confidence"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
	Tags       string  `json:"tags,omitempty"`
}

// Goal 计划模型（学习/项目/习惯/生活）
type Goal struct {
	ID            int    `json:"id"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	Type          string `json:"type"`   // learning / project / habit / life
	Status        string `json:"status"` // active / completed / paused / dropped
	StartDate     string `json:"start_date"`
	TargetDate    string `json:"target_date,omitempty"`
	CurrentFocus  string `json:"current_focus,omitempty"`
	NextStep      string `json:"next_step,omitempty"`
	CompanionNote string `json:"companion_note,omitempty"`
	Progress      int    `json:"progress"`
	Mood          string `json:"mood,omitempty"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// Milestone 里程碑
type Milestone struct {
	ID               int    `json:"id"`
	GoalID           int    `json:"goal_id"`
	Title            string `json:"title"`
	Description      string `json:"description,omitempty"`
	Status           string `json:"status"` // pending / active / completed
	CompletedAt      string `json:"completed_at,omitempty"`
	CompanionComment string `json:"companion_comment,omitempty"`
	OrderIndex       int    `json:"order_index"`
}

// CheckIn 记录/打卡（日记式）
type CheckIn struct {
	ID                int    `json:"id"`
	GoalID            int    `json:"goal_id"`
	Date              string `json:"date"`
	Content           string `json:"content"`
	Mood              string `json:"mood,omitempty"`
	CompanionResponse string `json:"companion_response,omitempty"`
	CreatedAt         string `json:"created_at"`
}

// Observation 观察模型
type Observation struct {
	ID        int    `json:"id"`
	Content   string `json:"content"`
	Type      string `json:"type"` // observation / reminder / milestone
	Displayed bool   `json:"displayed"`
	CreatedAt string `json:"created_at"`
}

// Highlight 高光回忆模型
type Highlight struct {
	ID          int    `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Date        string `json:"date"`
	MemoryIDs   string `json:"memory_ids,omitempty"`
	UserMarked  bool   `json:"user_marked"`
	CreatedAt   string `json:"created_at"`
}

// Reflection 复盘模型
type Reflection struct {
	ID                   int    `json:"id"`
	PeriodStart          string `json:"period_start"`
	PeriodEnd            string `json:"period_end"`
	GrowthAnalysis       string `json:"growth_analysis"`
	RelationshipAnalysis string `json:"relationship_analysis"`
	ProjectReview        string `json:"project_review"`
	Observations         string `json:"observations"`
	CreatedAt            string `json:"created_at"`
}

// Setting 设置模型
type Setting struct {
	ID        int    `json:"id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// MessageResponse 消息响应
type MessageResponse struct {
	Content        string `json:"content"`
	Agent          string `json:"agent"`
	AgentRoute     string `json:"agent_route,omitempty"`
	Emotion        string `json:"emotion,omitempty"`
	GoalSaved      string `json:"goal_saved,omitempty"`
	ResponseTimeMs int64  `json:"response_time_ms,omitempty"`
	Timestamp      string `json:"timestamp,omitempty"`
}

// CompanionStatus 伙伴状态
type CompanionStatus struct {
	Name       string `json:"name"`
	Mood       string `json:"mood"`
	LastSeen   string `json:"last_seen"`
	TrustLevel int    `json:"trust_level"`
}

// AutomationTask 自动化任务（替代 Schedule + ToolFlow）
type AutomationTask struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	TaskType         string `json:"task_type"`
	Config           string `json:"config"`
	ScheduleType     string `json:"schedule_type"`
	ScheduleConfig   string `json:"schedule_config"`
	Enabled          bool   `json:"enabled"`
	Status           string `json:"status"`
	LastRunAt        string `json:"last_run_at"`
	NextRunAt        string `json:"next_run_at"`
	MaxRetries       int    `json:"max_retries"`
	RetryIntervalSec int    `json:"retry_interval_sec"`
	SlashCommand     string `json:"slash_command"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// AutomationStep 流程编排步骤
type AutomationStep struct {
	ID            int    `json:"id"`
	TaskID        int    `json:"task_id"`
	StepIndex     int    `json:"step_index"`
	StepType      string `json:"step_type"`
	Name          string `json:"name"`
	Config        string `json:"config"`
	OutputVar     string `json:"output_var"`
	NextOnSuccess int    `json:"next_on_success"`
	NextOnFailure int    `json:"next_on_failure"`
}

// TaskTemplate 任务模板
type TaskTemplate struct {
	ID                    int    `json:"id"`
	Name                  string `json:"name"`
	Icon                  string `json:"icon"`
	Description           string `json:"description"`
	TaskType              string `json:"task_type"`
	DefaultConfig         string `json:"default_config"`
	DefaultScheduleType   string `json:"default_schedule_type"`
	DefaultScheduleConfig string `json:"default_schedule_config"`
	Steps                 string `json:"steps"`
	IsSystem              bool   `json:"is_system"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
}

// AutomationDependency 任务依赖关系
type AutomationDependency struct {
	ID          int    `json:"id"`
	TaskID      int    `json:"task_id"`
	DependsOnID int    `json:"depends_on_id"`
	Condition   string `json:"condition"`
	CreatedAt   string `json:"created_at"`
}

// AutomationExecution 任务执行记录
type AutomationExecution struct {
	ID            int    `json:"id"`
	TaskID        int    `json:"task_id"`
	Status        string `json:"status"`
	ResultType    string `json:"result_type"`
	ResultContent string `json:"result_content"`
	ResultPath    string `json:"result_path"`
	ErrorMessage  string `json:"error_message"`
	RetryCount    int    `json:"retry_count"`
	DurationMs    int64  `json:"duration_ms"`
	StartedAt     string `json:"started_at"`
	FinishedAt    string `json:"finished_at"`
}

// StepExecution 步骤级执行记录
type StepExecution struct {
	ID            int    `json:"id"`
	ExecutionID   int    `json:"execution_id"`
	StepIndex     int    `json:"step_index"`
	StepName      string `json:"step_name"`
	Status        string `json:"status"`
	InputPreview  string `json:"input_preview"`
	OutputPreview string `json:"output_preview"`
	DurationMs    int64  `json:"duration_ms"`
	ErrorMessage  string `json:"error_message"`
}

// TaskResult 任务执行结果
type TaskResult struct {
	Success    bool                   `json:"success"`
	StatusText string                 `json:"status_text"`
	ResultType string                 `json:"result_type"`
	Content    string                 `json:"content"`
	FilePath   string                 `json:"file_path"`
	Variables  map[string]interface{} `json:"variables"`
	Duration   int64                  `json:"duration"`
}

// ConfigField 配置字段定义（前端动态渲染表单用）
type ConfigField struct {
	Key         string         `json:"key"`
	Label       string         `json:"label"`
	Type        string         `json:"type"`
	Required    bool           `json:"required"`
	Default     string         `json:"default,omitempty"`
	Options     []ConfigOption `json:"options,omitempty"`
	Placeholder string         `json:"placeholder,omitempty"`
	Condition   string         `json:"condition,omitempty"`
}

// ConfigOption 配置选项
type ConfigOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}
