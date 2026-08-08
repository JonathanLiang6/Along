package scheduler

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"ai-companion/internal/models"
	"ai-companion/internal/services"

	"github.com/robfig/cron/v3"
)

// Scheduler 任务调度器 — 替代旧 Engine 的调度职责
type Scheduler struct {
	cron    *cron.Cron
	service *services.AutomationService
	db      *sql.DB
	dataDir string
	// 关键修复：使用 sync.Map 替代普通 map + sync.Mutex。
	// 原因：旧实现中 scheduleTask 持 s.mu 期间会注册 time.AfterFunc
	// 回调和 cron.AddFunc，其中 time.AfterFunc 的回调本身又需要
	// 获取 s.mu 来清理句柄。当 duration 极小（甚至为 0）的
	// once 任务、或 cron 内部 channel 发送短暂阻塞时，会形成
	// "持锁 goroutine 等待自己注册的回调释放锁" 的不可恢复死锁，
	// 进而导致 app.startup 永久阻塞、UI 一直黑屏。
	// 改用 sync.Map 后，scheduleTask 不再持任何外层锁，所有句柄
	// 操作都是原子的 LoadAndDelete / Store，从根本上消除此死锁。
	cronJobs      sync.Map // map[int]cron.EntryID
	oneShotTimers sync.Map // map[int]*time.Timer

	// 回调：执行Agent类任务（接收 execID 避免重复创建执行记录）
	OnExecuteAgentTask func(execID int, task *models.AutomationTask) *models.AutomationExecution `json:"-"`
}

// New 创建调度器
// 修复 #23：AutomationService 改为由外部注入（app.startup 持有的同一实例），
// 避免出现"两个 AutomationService 操作同一 sql.DB"导致的重复或状态漂移。
// 修复 #22：维护 oneShotTimers 句柄，让 UnscheduleTask / Stop 能取消未触发的 timer。
func New(db *sql.DB, dataDir string, service *services.AutomationService) *Scheduler {
	// cron.Recover 会在每次任务触发时捕获 panic，避免一个坏任务
	// 拖垮整个调度器进程。cron.Recover 需要 cron.Logger 接口，
	// 用 cron.PrintfLogger 把标准 log 包适配过去。
	if service == nil {
		// 安全网：调用方忘记传时降级为内部新建，保证调度器仍能工作。
		// 生产路径（app.startup）始终会显式注入。
		service = services.NewAutomationService(db)
	}
	return &Scheduler{
		cron: cron.New(
			cron.WithLocation(time.Local),
			cron.WithChain(cron.Recover(cron.PrintfLogger(log.Default()))),
		),
		service:       service,
		db:            db,
		dataDir:       dataDir,
		// cronJobs / oneShotTimers 是 sync.Map，零值即可用，无需 make
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
// 修复 #22：同时停掉所有未触发的一次性 timer。
// 关键修复：使用 sync.Map.Range + LoadAndDelete，避免在持锁
// 状态下遍历并删除 map 元素，且与一次性 timer 回调内的
// LoadAndDelete 天然无竞争（sync.Map 内部已做并发安全）。
func (s *Scheduler) Stop() {
	s.oneShotTimers.Range(func(key, value interface{}) bool {
		if t, ok := value.(*time.Timer); ok && t != nil {
			t.Stop()
		}
		s.oneShotTimers.Delete(key)
		return true
	})
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

// UnscheduleTask 取消调度（同时取消一次性任务的 timer）
// 关键修复：用 sync.Map 的 LoadAndDelete 替代"持锁 + 读 map + 删除"，
// 既避免与一次性 timer 回调中的清理动作产生锁竞争，也消除了
// "持锁期间反向等待 s.mu 的回调" 形成的死锁路径。
func (s *Scheduler) UnscheduleTask(taskID int) {
	if v, ok := s.cronJobs.LoadAndDelete(taskID); ok {
		if entryID, ok := v.(cron.EntryID); ok {
			s.cron.Remove(entryID)
		}
	}
	if v, ok := s.oneShotTimers.LoadAndDelete(taskID); ok {
		if t, ok := v.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
}

func (s *Scheduler) scheduleTask(task *models.AutomationTask) error {
	// 关键修复：取消旧调度（无论是否启用，都先清掉旧 entry/timer）
	s.cleanupTask(task.ID)

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
			// 关键修复：time.AfterFunc 的回调不再获取 s.mu，
			// 而是用 sync.Map 的 Delete 自带并发安全。即使
			// duration 极小、回调在 scheduleTask 返回前被
			// runtime 调度，也不会再有"持锁 goroutine 等待
			// 自己注册的回调"这种死锁。
			timer := time.AfterFunc(duration, func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[Scheduler] 一次性任务 %d 执行 panic: %v", taskID, r)
					}
				}()
				// 触发后清理句柄映射（timer 已自然完成）
				s.oneShotTimers.Delete(taskID)
				s.ExecuteTask(taskID)
			})
			s.oneShotTimers.Store(task.ID, timer)
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
	// 关键修复：不再持 s.mu 锁调用 AddFunc，消除"持锁 + 阻塞在
	// cron 内部 channel 发送"导致的启动期长时间卡顿/黑屏。
	// scheduleTask 是单线程串行调用（Start/ScheduleTask 入口），
	// AddFunc 内部对 cron.runningMu 的获取与此处不冲突。
	entryID, err := s.cron.AddFunc(cronExpr, func() {
		s.ExecuteTask(taskID)
	})
	if err != nil {
		return err
	}

	s.cronJobs.Store(task.ID, entryID)
	_ = s.updateNextRunTime(task, cronExpr)
	return nil
}

// cleanupTask 取消指定任务的旧调度（cron entry 与 one-shot timer）
// 内部使用 sync.Map.LoadAndDelete 原子完成"读 + 删"，对并发
// 调用方（包括一次性 timer 回调）天然安全。
func (s *Scheduler) cleanupTask(taskID int) {
	if v, ok := s.cronJobs.LoadAndDelete(taskID); ok {
		if entryID, ok := v.(cron.EntryID); ok {
			s.cron.Remove(entryID)
		}
	}
	if v, ok := s.oneShotTimers.LoadAndDelete(taskID); ok {
		if t, ok := v.(*time.Timer); ok && t != nil {
			t.Stop()
		}
	}
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
// 入口处通过 defer recover 兜底：即便 Agent 任务内的 panic 漏过
// cron.Recover，也能保证单次执行失败被记录而不是拖垮进程。
func (s *Scheduler) ExecuteTask(taskID int) (exec *models.AutomationExecution) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("[Scheduler] 任务 %d 执行 panic: %v", taskID, r)
			exec = &models.AutomationExecution{
				TaskID:       taskID,
				Status:       "failed",
				ErrorMessage: fmt.Sprintf("任务执行 panic: %v", r),
			}
		}
	}()

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
			exec := s.OnExecuteAgentTask(execID, task)
			result = models.TaskResult{
				Success:    exec.Status == "success",
				StatusText: exec.Status,
				ResultType: exec.ResultType,
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
// 解析失败时打印日志，避免上游拿到错误的"空配置"还误以为配置正确。
func parseConfig(configJSON string) map[string]interface{} {
	cfg := make(map[string]interface{})
	if configJSON == "" {
		return cfg
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		log.Printf("[parseConfig] 解析配置失败: %v (raw=%q)", err, configJSON)
		cfg = make(map[string]interface{})
	}
	return cfg
}
