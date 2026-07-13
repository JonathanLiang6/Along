package automation

import (
	"database/sql"

	"ai-companion/internal/agents"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// TaskContext 任务执行上下文
type TaskContext struct {
	TaskID    int
	TaskName  string
	TaskType  string
	Variables map[string]interface{}
	DataDir   string
	DB        *sql.DB
	AgentMgr  *agents.AgentManager
	ExecID    int
}

// TaskExecutor 任务执行器接口
type TaskExecutor interface {
	Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error)
	ConfigSchema() []models.ConfigField
}

// ExecutorRegistry 执行器注册表
type ExecutorRegistry struct {
	executors map[string]TaskExecutor
}

// NewExecutorRegistry 创建注册表并注册所有执行器
func NewExecutorRegistry(agentMgr *agents.AgentManager, db *sql.DB, dataDir string) *ExecutorRegistry {
	registry := &ExecutorRegistry{
		executors: make(map[string]TaskExecutor),
	}

	// 注册10种执行器
	registry.Register("agent_chat", &AgentChatExecutor{agentMgr: agentMgr})
	registry.Register("web_search", &WebSearchExecutor{agentMgr: agentMgr})
	registry.Register("report", &ReportExecutor{agentMgr: agentMgr, db: db, dataDir: dataDir})
	registry.Register("backup", &BackupExecutor{db: db, dataDir: dataDir})
	registry.Register("reminder", &ReminderExecutor{})
	registry.Register("monitor", &MonitorExecutor{agentMgr: agentMgr})
	registry.Register("habit_checkin", &HabitCheckinExecutor{agentMgr: agentMgr, db: db})
	registry.Register("review", &ReviewExecutor{agentMgr: agentMgr, db: db})
	registry.Register("cleanup", &CleanupExecutor{db: db})
	registry.Register("workflow", &WorkflowExecutor{agentMgr: agentMgr, db: db, dataDir: dataDir})

	return registry
}

// Register 注册执行器
func (r *ExecutorRegistry) Register(taskType string, executor TaskExecutor) {
	r.executors[taskType] = executor
}

// Get 获取执行器
func (r *ExecutorRegistry) Get(taskType string) (TaskExecutor, bool) {
	executor, ok := r.executors[taskType]
	return executor, ok
}

// GetSchema 获取任务类型配置Schema
func (r *ExecutorRegistry) GetSchema(taskType string) []models.ConfigField {
	if executor, ok := r.executors[taskType]; ok {
		return executor.ConfigSchema()
	}
	return nil
}

// GetAllSchemas 获取所有任务类型的Schema
func (r *ExecutorRegistry) GetAllSchemas() map[string][]models.ConfigField {
	schemas := make(map[string][]models.ConfigField)
	for taskType, executor := range r.executors {
		schemas[taskType] = executor.ConfigSchema()
	}
	return schemas
}

// truncate 截断字符串
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// 确保services包被使用
var _ = services.NewAutomationService
