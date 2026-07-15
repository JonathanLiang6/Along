package scheduler

import (
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"ai-companion/internal/models"
	"ai-companion/internal/services"

	"github.com/robfig/cron/v3"
)

// Scheduler 任务调度器 — 替代旧 Engine 的调度职责
type Scheduler struct {
	cron     *cron.Cron
	service  *services.AutomationService
	db       *sql.DB
	dataDir  string
	cronJobs map[int]cron.EntryID
	mu       sync.Mutex

	// 回调：执行Agent类任务
	OnExecuteAgentTask func(task *models.AutomationTask) *models.AutomationExecution `json:"-"`
}

// New 创建调度器
func New(db *sql.DB, dataDir string) *Scheduler {
	return &Scheduler{
		cron:     cron.New(cron.WithLocation(time.Local)),
		service:  services.NewAutomationService(db),
		db:       db,
		dataDir:  dataDir,
		cronJobs: make(map[int]cron.EntryID),
	}
}

// Start 启动调度器，加载所有已启用任务
func (s *Scheduler) Start() error {
	s.cron.Start()

	tasks, err := s.service.GetTasks("")
	if err != nil {
		return err
	}

	for _, task := range tasks {
		if task.Enabled {
			if err := s.scheduleTask(&task); err != nil {
				log.Printf("调度任务 %d (%s) 失败: %v", task.ID, task.Name, err)
			}
		}
	}

	log.Printf("调度器已启动，加载了 %d 个任务", len(tasks))
	return nil
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// ScheduleTask 调度/重新调度单个任务
func (s *Scheduler) ScheduleTask(taskID int) error {
	task, err := s.service.GetTask(taskID)
	if err != nil {
		return err
	}
	return s.scheduleTask(task)
}

// UnscheduleTask 取消调度
func (s *Scheduler) UnscheduleTask(taskID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if entryID, ok := s.cronJobs[taskID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronJobs, taskID)
	}
}

func (s *Scheduler) scheduleTask(task *models.AutomationTask) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 移除旧调度
	if entryID, ok := s.cronJobs[task.ID]; ok {
		s.cron.Remove(entryID)
		delete(s.cronJobs, task.ID)
	}

	if !task.Enabled {
		return nil
	}

	// 一次性任务
	if task.ScheduleType == "once" {
		execTime, err := services.GetOnceTime(task.ScheduleConfig)
		if err != nil {
			return err
		}
		duration := time.Until(execTime)
		if duration > 0 {
			taskID := task.ID
			time.AfterFunc(duration, func() {
				s.ExecuteTask(taskID)
			})
		}
		s.service.UpdateTaskRunTime(task.ID, "", execTime.Format("2006-01-02 15:04:05"))
		return nil
	}

	// Cron 调度
	cronExpr, err := services.ParseScheduleConfig(task.ScheduleType, task.ScheduleConfig)
	if err != nil {
		return err
	}

	taskID := task.ID
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.ExecuteTask(taskID)
	})
	if err != nil {
		return err
	}

	s.cronJobs[task.ID] = entryID
	_ = s.updateNextRunTime(task, cronExpr)
	return nil
}

func (s *Scheduler) updateNextRunTime(task *models.AutomationTask, cronExpr string) error {
	sched, err := cron.ParseStandard(cronExpr)
	if err != nil {
		return err
	}
	nextRun := sched.Next(time.Now())
	return s.service.UpdateTaskRunTime(task.ID, "", nextRun.Format("2006-01-02 15:04:05"))
}

// ExecuteTask 执行单个任务（公共方法）
func (s *Scheduler) ExecuteTask(taskID int) *models.AutomationExecution {
	task, err := s.service.GetTask(taskID)
	if err != nil {
		log.Printf("获取任务 %d 失败: %v", taskID, err)
		return nil
	}

	execID, err := s.service.CreateExecution(taskID)
	if err != nil {
		log.Printf("创建执行记录失败: %v", err)
		return nil
	}

	s.service.UpdateTaskStatus(taskID, "running")
	startTime := time.Now()

	var result models.TaskResult

	// 系统任务：直接执行
	switch task.TaskType {
	case "backup":
		result = executeBackup(task.Config, s.dataDir)
	case "cleanup":
		result = executeCleanup(task.Config, s.db)
	default:
		// Agent 相关任务：通过回调交给 Orchestrator
		if s.OnExecuteAgentTask != nil {
			exec := s.OnExecuteAgentTask(task)
			result = models.TaskResult{
				Success:    exec.Status == "success",
				StatusText: exec.Status,
				Content:    exec.ResultContent,
				FilePath:   exec.ResultPath,
			}
		} else {
			result = models.TaskResult{
				Success:    false,
				StatusText: "Orchestrator 未配置",
			}
		}
	}

	// 更新执行记录
	status := "success"
	errMsg := ""
	if !result.Success {
		status = "failed"
		errMsg = result.StatusText
	}

	duration := time.Since(startTime).Milliseconds()
	s.service.UpdateExecution(execID, status, result.ResultType, result.Content, result.FilePath, errMsg, duration)
	s.service.UpdateTaskStatus(taskID, status)

	// 更新时间
	now := time.Now()
	lastRun := now.Format("2006-01-02 15:04:05")
	nextRun := ""
	if task.ScheduleType != "once" {
		cronExpr, _ := services.ParseScheduleConfig(task.ScheduleType, task.ScheduleConfig)
		if sched, err := cron.ParseStandard(cronExpr); err == nil {
			nextRun = sched.Next(now).Format("2006-01-02 15:04:05")
		}
	}
	s.service.UpdateTaskRunTime(taskID, lastRun, nextRun)

	// 触发依赖
	s.triggerDependents(taskID, status)

	return &models.AutomationExecution{
		ID:            execID,
		TaskID:        taskID,
		Status:        status,
		ResultContent: result.Content,
		ResultPath:    result.FilePath,
		DurationMs:    duration,
		StartedAt:     startTime.Format("2006-01-02 15:04:05"),
		FinishedAt:    now.Format("2006-01-02 15:04:05"),
	}
}

// triggerDependents 触发依赖任务
func (s *Scheduler) triggerDependents(taskID int, status string) {
	dependents, err := s.service.GetDependents(taskID)
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
			go s.ExecuteTask(dep.TaskID)
		}
	}
}

// GetService 获取自动化服务
func (s *Scheduler) GetService() *services.AutomationService {
	return s.service
}

// parseConfig 解析任务配置 JSON
func parseConfig(configJSON string) map[string]interface{} {
	var cfg map[string]interface{}
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	return cfg
}
