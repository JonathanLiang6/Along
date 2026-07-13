package automation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/models"
	"ai-companion/internal/services"

	"github.com/robfig/cron/v3"
)

// Engine 统一调度引擎
type Engine struct {
	cron     *cron.Cron
	registry *ExecutorRegistry
	service  *services.AutomationService
	agentMgr *agents.AgentManager
	db       *sql.DB
	dataDir  string
	cronJobs map[int]cron.EntryID // taskID -> cron entry
	mu       sync.Mutex
}

// NewEngine 创建调度引擎
func NewEngine(db *sql.DB, agentMgr *agents.AgentManager, dataDir string) *Engine {
	return &Engine{
		cron:     cron.New(cron.WithLocation(time.Local)),
		registry: NewExecutorRegistry(agentMgr, db, dataDir),
		service:  services.NewAutomationService(db),
		agentMgr: agentMgr,
		db:       db,
		dataDir:  dataDir,
		cronJobs: make(map[int]cron.EntryID),
	}
}

// Start 启动引擎，加载所有已启用任务
func (e *Engine) Start() error {
	e.cron.Start()

	// 加载所有已启用任务
	tasks, err := e.service.GetTasks("")
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Enabled {
			if err := e.scheduleTask(&task); err != nil {
				log.Printf("调度任务 %d (%s) 失败: %v", task.ID, task.Name, err)
			}
		}
	}

	log.Printf("自动化引擎已启动，加载了 %d 个任务", len(tasks))
	return nil
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.cron.Stop()
}

// scheduleTask 调度单个任务
func (e *Engine) scheduleTask(task *models.AutomationTask) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	// 如果已有调度，先移除
	if entryID, ok := e.cronJobs[task.ID]; ok {
		e.cron.Remove(entryID)
		delete(e.cronJobs, task.ID)
	}

	if !task.Enabled {
		return nil
	}

	// 一次性任务特殊处理
	if task.ScheduleType == "once" {
		execTime, err := services.GetOnceTime(task.ScheduleConfig)
		if err != nil {
			return err
		}
		duration := time.Until(execTime)
		if duration > 0 {
			taskID := task.ID
			time.AfterFunc(duration, func() {
				e.ExecuteTask(taskID)
			})
		}
		e.service.UpdateTaskRunTime(task.ID, "", execTime.Format("2006-01-02 15:04:05"))
		return nil
	}

	// 解析cron表达式
	cronExpr, err := services.ParseScheduleConfig(task.ScheduleType, task.ScheduleConfig)
	if err != nil {
		return err
	}

	taskID := task.ID
	entryID, err := e.cron.AddFunc(cronExpr, func() {
		e.ExecuteTask(taskID)
	})
	if err != nil {
		return err
	}

	e.cronJobs[task.ID] = entryID

	// 计算下次执行时间
	_ = e.updateNextRunTime(task, cronExpr)

	return nil
}

// updateNextRunTime 更新下次执行时间
func (e *Engine) updateNextRunTime(task *models.AutomationTask, cronExpr string) error {
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return err
	}
	nextRun := sched.Next(time.Now())
	return e.service.UpdateTaskRunTime(task.ID, "", nextRun.Format("2006-01-02 15:04:05"))
}

// ScheduleTask 重新调度任务（供外部调用）
func (e *Engine) ScheduleTask(taskID int) error {
	task, err := e.service.GetTask(taskID)
	if err != nil {
		return err
	}
	return e.scheduleTask(task)
}

// UnscheduleTask 取消调度
func (e *Engine) UnscheduleTask(taskID int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if entryID, ok := e.cronJobs[taskID]; ok {
		e.cron.Remove(entryID)
		delete(e.cronJobs, taskID)
	}
}

// ExecuteTask 执行任务（核心方法）
func (e *Engine) ExecuteTask(taskID int) *models.AutomationExecution {
	task, err := e.service.GetTask(taskID)
	if err != nil {
		log.Printf("获取任务 %d 失败: %v", taskID, err)
		return nil
	}

	// 创建执行记录
	execID, err := e.service.CreateExecution(taskID)
	if err != nil {
		log.Printf("创建执行记录失败: %v", err)
		return nil
	}

	e.service.UpdateTaskStatus(taskID, "running")

	startTime := time.Now()
	ctx := TaskContext{
		TaskID:    taskID,
		TaskName:  task.Name,
		TaskType:  task.TaskType,
		Variables: make(map[string]interface{}),
		DataDir:   e.dataDir,
		DB:        e.db,
		AgentMgr:  e.agentMgr,
		ExecID:    execID,
	}

	// 解析config
	var config map[string]interface{}
	if task.Config != "" {
		json.Unmarshal([]byte(task.Config), &config)
	}
	if config == nil {
		config = make(map[string]interface{})
	}

	// 获取执行器
	executor, ok := e.registry.Get(task.TaskType)
	if !ok {
		errMsg := fmt.Sprintf("未知任务类型: %s", task.TaskType)
		e.service.UpdateExecution(execID, "failed", "none", "", "", errMsg, time.Since(startTime).Milliseconds())
		e.service.UpdateTaskStatus(taskID, "failed")
		return nil
	}

	// 执行（含重试逻辑）
	result := e.executeWithRetry(executor, config, ctx, task, execID, startTime)

	// 更新执行记录
	status := "success"
	errMsg := ""
	if !result.Success {
		status = "failed"
		errMsg = result.StatusText
	}

	e.service.UpdateExecution(execID, status, result.ResultType, result.Content, result.FilePath, errMsg, result.Duration)
	e.service.UpdateTaskStatus(taskID, status)

	// 更新执行时间
	now := time.Now()
	lastRun := now.Format("2006-01-02 15:04:05")
	nextRun := ""
	if task.ScheduleType != "once" {
		cronExpr, _ := services.ParseScheduleConfig(task.ScheduleType, task.ScheduleConfig)
		if sched, err := cron.ParseStandard(cronExpr); err == nil {
			nextRun = sched.Next(now).Format("2006-01-02 15:04:05")
		}
	}
	e.service.UpdateTaskRunTime(taskID, lastRun, nextRun)

	// 检查依赖触发
	e.triggerDependents(taskID, status)

	return &models.AutomationExecution{
		ID:            execID,
		TaskID:        taskID,
		Status:        status,
		ResultType:    result.ResultType,
		ResultContent: result.Content,
		ResultPath:    result.FilePath,
		DurationMs:    result.Duration,
		StartedAt:     startTime.Format("2006-01-02 15:04:05"),
		FinishedAt:    now.Format("2006-01-02 15:04:05"),
	}
}

// executeWithRetry 带重试的执行
func (e *Engine) executeWithRetry(executor TaskExecutor, config map[string]interface{}, ctx TaskContext, task *models.AutomationTask, execID int, startTime time.Time) *models.TaskResult {
	maxRetries := task.MaxRetries
	retryInterval := time.Duration(task.RetryIntervalSec) * time.Second

	var lastResult *models.TaskResult
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			e.service.UpdateExecutionRetry(execID, attempt)
			log.Printf("任务 %s 第%d次重试...", task.Name, attempt)
			time.Sleep(retryInterval)
		}

		result, err := executor.Execute(config, ctx)
		if err != nil {
			lastResult = &models.TaskResult{
				Success:    false,
				StatusText: err.Error(),
				Duration:   time.Since(startTime).Milliseconds(),
			}
			continue
		}

		if result.Success {
			return result
		}
		lastResult = result
	}

	if lastResult == nil {
		lastResult = &models.TaskResult{
			Success:    false,
			StatusText: "执行失败",
			Duration:   time.Since(startTime).Milliseconds(),
		}
	}
	return lastResult
}

// triggerDependents 触发依赖任务
func (e *Engine) triggerDependents(taskID int, status string) {
	dependents, err := e.service.GetDependents(taskID)
	if err != nil {
		return
	}

	for _, dep := range dependents {
		shouldTrigger := false
		switch dep.Condition {
		case "on_success":
			shouldTrigger = status == "success"
		case "on_failure":
			shouldTrigger = status == "failed"
		case "on_complete":
			shouldTrigger = true
		}

		if shouldTrigger {
			log.Printf("触发依赖任务 %d (条件: %s)", dep.TaskID, dep.Condition)
			go e.ExecuteTask(dep.TaskID)
		}
	}
}

// GetRegistry 获取执行器注册表
func (e *Engine) GetRegistry() *ExecutorRegistry {
	return e.registry
}

// GetService 获取服务
func (e *Engine) GetService() *services.AutomationService {
	return e.service
}
