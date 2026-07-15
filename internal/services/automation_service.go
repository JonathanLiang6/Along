package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"ai-companion/internal/models"
)

// AutomationService 自动化任务服务
type AutomationService struct {
	db *sql.DB
}

// NewAutomationService 创建自动化任务服务
func NewAutomationService(db *sql.DB) *AutomationService {
	return &AutomationService{db: db}
}

// GetTasks 获取任务列表，taskType为空时返回全部
func (s *AutomationService) GetTasks(taskType string) ([]models.AutomationTask, error) {
	var query string
	var args []interface{}
	if taskType == "" {
		query = `SELECT id, name, COALESCE(description, ''), task_type, config, schedule_type, schedule_config,
					enabled, COALESCE(status, ''), COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
					max_retries, retry_interval_sec, COALESCE(slash_command, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
				 FROM automation_tasks ORDER BY id DESC`
	} else {
		query = `SELECT id, name, COALESCE(description, ''), task_type, config, schedule_type, schedule_config,
					enabled, COALESCE(status, ''), COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
					max_retries, retry_interval_sec, COALESCE(slash_command, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
				 FROM automation_tasks WHERE task_type = ? ORDER BY id DESC`
		args = append(args, taskType)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []models.AutomationTask
	for rows.Next() {
		task, err := scanTaskRow(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

// GetTask 获取单个任务
func (s *AutomationService) GetTask(id int) (*models.AutomationTask, error) {
	query := `SELECT id, name, COALESCE(description, ''), task_type, config, schedule_type, schedule_config,
				enabled, COALESCE(status, ''), COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
				max_retries, retry_interval_sec, COALESCE(slash_command, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
			  FROM automation_tasks WHERE id = ?`

	row := s.db.QueryRow(query, id)
	task, err := scanTaskRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("任务 %d 不存在", id)
		}
		return nil, err
	}
	return &task, nil
}

// CreateTask 创建任务
func (s *AutomationService) CreateTask(task *models.AutomationTask) (int, error) {
	query := `INSERT INTO automation_tasks (name, description, task_type, config, schedule_type, schedule_config,
				enabled, status, max_retries, retry_interval_sec, slash_command)
			  VALUES (?, ?, ?, ?, ?, ?, ?, 'idle', ?, ?, ?)`
	result, err := s.db.Exec(query, task.Name, task.Description, task.TaskType, task.Config,
		task.ScheduleType, task.ScheduleConfig, task.Enabled, task.MaxRetries, task.RetryIntervalSec, task.SlashCommand)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// UpdateTask 更新任务
func (s *AutomationService) UpdateTask(task *models.AutomationTask) error {
	query := `UPDATE automation_tasks SET name = ?, description = ?, task_type = ?, config = ?,
				schedule_type = ?, schedule_config = ?, enabled = ?, max_retries = ?, retry_interval_sec = ?,
				slash_command = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := s.db.Exec(query, task.Name, task.Description, task.TaskType, task.Config,
		task.ScheduleType, task.ScheduleConfig, task.Enabled, task.MaxRetries, task.RetryIntervalSec, task.SlashCommand, task.ID)
	return err
}

// DeleteTask 删除任务
func (s *AutomationService) DeleteTask(id int) error {
	_, err := s.db.Exec("DELETE FROM automation_tasks WHERE id = ?", id)
	return err
}

// UpdateTaskStatus 更新任务状态
func (s *AutomationService) UpdateTaskStatus(taskID int, status string) error {
	_, err := s.db.Exec("UPDATE automation_tasks SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", status, taskID)
	return err
}

// UpdateTaskRunTime 更新任务执行时间
func (s *AutomationService) UpdateTaskRunTime(taskID int, lastRun, nextRun string) error {
	_, err := s.db.Exec("UPDATE automation_tasks SET last_run_at = ?, next_run_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		lastRun, nextRun, taskID)
	return err
}

// CreateExecution 创建执行记录
func (s *AutomationService) CreateExecution(taskID int) (int, error) {
	query := `INSERT INTO automation_executions (task_id, status, started_at) VALUES (?, 'running', datetime('now'))`
	result, err := s.db.Exec(query, taskID)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// UpdateExecution 更新执行记录
func (s *AutomationService) UpdateExecution(execID int, status, resultType, content, filePath, errMsg string, durationMs int64) error {
	query := `UPDATE automation_executions SET status = ?, result_type = ?, result_content = ?, result_path = ?,
				error_message = ?, duration_ms = ?, finished_at = datetime('now') WHERE id = ?`
	_, err := s.db.Exec(query, status, resultType, content, filePath, errMsg, durationMs, execID)
	return err
}

// UpdateExecutionRetry 更新重试次数
func (s *AutomationService) UpdateExecutionRetry(execID int, attempt int) error {
	_, err := s.db.Exec("UPDATE automation_executions SET retry_count = ? WHERE id = ?", attempt, execID)
	return err
}

// GetDependents 获取依赖指定任务的任务列表
func (s *AutomationService) GetDependents(taskID int) ([]models.AutomationDependency, error) {
	query := `SELECT id, task_id, depends_on_id, condition, COALESCE(created_at, '')
			  FROM automation_dependencies WHERE depends_on_id = ?`
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []models.AutomationDependency
	for rows.Next() {
		var d models.AutomationDependency
		if err := rows.Scan(&d.ID, &d.TaskID, &d.DependsOnID, &d.Condition, &d.CreatedAt); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// GetSteps 获取任务的步骤列表
func (s *AutomationService) GetSteps(taskID int) ([]models.AutomationStep, error) {
	query := `SELECT id, task_id, step_index, step_type, name, config, output_var, next_on_success, next_on_failure
			  FROM automation_steps WHERE task_id = ? ORDER BY step_index ASC`
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []models.AutomationStep
	for rows.Next() {
		var st models.AutomationStep
		if err := rows.Scan(&st.ID, &st.TaskID, &st.StepIndex, &st.StepType, &st.Name, &st.Config,
			&st.OutputVar, &st.NextOnSuccess, &st.NextOnFailure); err != nil {
			continue
		}
		steps = append(steps, st)
	}
	return steps, nil
}

// CreateStepExecution 创建步骤执行记录
func (s *AutomationService) CreateStepExecution(execID, stepIndex int, name string) (int, error) {
	query := `INSERT INTO automation_step_executions (execution_id, step_index, step_name, status)
			  VALUES (?, ?, ?, 'running')`
	result, err := s.db.Exec(query, execID, stepIndex, name)
	if err != nil {
		return 0, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return int(id), nil
}

// UpdateStepExecution 更新步骤执行记录
func (s *AutomationService) UpdateStepExecution(stepExecID int, status, inputPreview, outputPreview, errMsg string, durationMs int64) error {
	query := `UPDATE automation_step_executions SET status = ?, input_preview = ?, output_preview = ?,
				error_message = ?, duration_ms = ? WHERE id = ?`
	_, err := s.db.Exec(query, status, inputPreview, outputPreview, errMsg, durationMs, stepExecID)
	return err
}

// ToggleTask 启用/禁用任务
func (s *AutomationService) ToggleTask(taskID int, enabled bool) error {
	_, err := s.db.Exec("UPDATE automation_tasks SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", enabled, taskID)
	return err
}

// GetExecutions 获取任务执行记录
func (s *AutomationService) GetExecutions(taskID int) ([]models.AutomationExecution, error) {
	query := `SELECT id, task_id, status, COALESCE(result_type, ''), COALESCE(result_content, ''),
				COALESCE(result_path, ''), COALESCE(error_message, ''), retry_count, duration_ms,
				COALESCE(started_at, ''), COALESCE(finished_at, '')
			  FROM automation_executions WHERE task_id = ? ORDER BY id DESC`
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var execs []models.AutomationExecution
	for rows.Next() {
		var e models.AutomationExecution
		if err := rows.Scan(&e.ID, &e.TaskID, &e.Status, &e.ResultType, &e.ResultContent,
			&e.ResultPath, &e.ErrorMessage, &e.RetryCount, &e.DurationMs, &e.StartedAt, &e.FinishedAt); err != nil {
			continue
		}
		execs = append(execs, e)
	}
	return execs, nil
}

// GetStepExecutions 获取步骤执行记录
func (s *AutomationService) GetStepExecutions(executionID int) ([]models.StepExecution, error) {
	query := `SELECT id, execution_id, step_index, step_name, status,
				COALESCE(input_preview, ''), COALESCE(output_preview, ''), duration_ms,
				COALESCE(error_message, '')
			  FROM automation_step_executions WHERE execution_id = ? ORDER BY step_index ASC`
	rows, err := s.db.Query(query, executionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var steps []models.StepExecution
	for rows.Next() {
		var st models.StepExecution
		if err := rows.Scan(&st.ID, &st.ExecutionID, &st.StepIndex, &st.StepName, &st.Status,
			&st.InputPreview, &st.OutputPreview, &st.DurationMs, &st.ErrorMessage); err != nil {
			continue
		}
		steps = append(steps, st)
	}
	return steps, nil
}

// GetDependencies 获取任务的依赖关系（该任务依赖哪些任务）
func (s *AutomationService) GetDependencies(taskID int) ([]models.AutomationDependency, error) {
	query := `SELECT id, task_id, depends_on_id, condition, COALESCE(created_at, '')
			  FROM automation_dependencies WHERE task_id = ?`
	rows, err := s.db.Query(query, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var deps []models.AutomationDependency
	for rows.Next() {
		var d models.AutomationDependency
		if err := rows.Scan(&d.ID, &d.TaskID, &d.DependsOnID, &d.Condition, &d.CreatedAt); err != nil {
			continue
		}
		deps = append(deps, d)
	}
	return deps, nil
}

// AddDependency 添加任务依赖
func (s *AutomationService) AddDependency(taskID, dependsOnID int, condition string) error {
	_, err := s.db.Exec(`INSERT INTO automation_dependencies (task_id, depends_on_id, condition) VALUES (?, ?, ?)`,
		taskID, dependsOnID, condition)
	return err
}

// RemoveDependency 删除任务依赖
func (s *AutomationService) RemoveDependency(id int) error {
	_, err := s.db.Exec("DELETE FROM automation_dependencies WHERE id = ?", id)
	return err
}

// SaveStepsJSON 保存workflow步骤（JSON格式）
func (s *AutomationService) SaveStepsJSON(taskID int, stepsJSON string) error {
	// 删除现有步骤
	_, err := s.db.Exec("DELETE FROM automation_steps WHERE task_id = ?", taskID)
	if err != nil {
		return err
	}

	if stepsJSON == "" {
		return nil
	}

	// 解析步骤JSON
	var steps []struct {
		StepIndex     int    `json:"step_index"`
		StepType      string `json:"step_type"`
		Name          string `json:"name"`
		Config        string `json:"config"`
		OutputVar     string `json:"output_var"`
		NextOnSuccess int    `json:"next_on_success"`
		NextOnFailure int    `json:"next_on_failure"`
	}
	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return fmt.Errorf("解析步骤JSON失败: %w", err)
	}

	for _, step := range steps {
		_, err := s.db.Exec(`INSERT INTO automation_steps (task_id, step_index, step_type, name, config, output_var, next_on_success, next_on_failure)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			taskID, step.StepIndex, step.StepType, step.Name, step.Config, step.OutputVar, step.NextOnSuccess, step.NextOnFailure)
		if err != nil {
			return err
		}
	}
	return nil
}

// ==================== 辅助函数 ====================

// scanTaskRow 通用行扫描（同时支持 *sql.Rows 和 *sql.Row）
type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanTaskRow(r rowScanner) (models.AutomationTask, error) {
	var t models.AutomationTask
	err := r.Scan(&t.ID, &t.Name, &t.Description, &t.TaskType, &t.Config, &t.ScheduleType,
		&t.ScheduleConfig, &t.Enabled, &t.Status, &t.LastRunAt, &t.NextRunAt,
		&t.MaxRetries, &t.RetryIntervalSec, &t.SlashCommand, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return t, err
	}
	return t, nil
}

// ParseScheduleConfig 解析调度配置，返回cron表达式
func ParseScheduleConfig(scheduleType, config string) (string, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return "", fmt.Errorf("解析调度配置失败: %w", err)
	}

	switch scheduleType {
	case "custom":
		cron, _ := cfg["cron"].(string)
		if cron == "" {
			return "", fmt.Errorf("custom类型需要cron字段")
		}
		return cron, nil

	case "daily":
		timeStr, _ := cfg["time"].(string)
		if timeStr == "" {
			timeStr = "09:00"
		}
		hour, minute := parseTimeStr(timeStr)
		return fmt.Sprintf("%d %d * * *", minute, hour), nil

	case "weekly":
		timeStr, _ := cfg["time"].(string)
		if timeStr == "" {
			timeStr = "09:00"
		}
		hour, minute := parseTimeStr(timeStr)
		if daysArr, ok := cfg["days"].([]interface{}); ok && len(daysArr) > 0 {
			var days []string
			for _, d := range daysArr {
				days = append(days, fmt.Sprintf("%d", int(d.(float64))))
			}
			return fmt.Sprintf("%d %d * * %s", minute, hour, strings.Join(days, ",")), nil
		}
		dayOfWeek := 1
		if d, ok := cfg["day"].(float64); ok {
			dayOfWeek = int(d)
		}
		return fmt.Sprintf("%d %d * * %d", minute, hour, dayOfWeek), nil

	case "monthly":
		timeStr, _ := cfg["time"].(string)
		if timeStr == "" {
			timeStr = "09:00"
		}
		hour, minute := parseTimeStr(timeStr)
		dayOfMonth := 1
		if d, ok := cfg["day"].(float64); ok {
			dayOfMonth = int(d)
		}
		return fmt.Sprintf("%d %d %d * *", minute, hour, dayOfMonth), nil

	case "interval":
		seconds := 300
		if s, ok := cfg["seconds"].(float64); ok {
			seconds = int(s)
		}
		minutes := seconds / 60
		if minutes < 1 {
			minutes = 1
		}
		return fmt.Sprintf("*/%d * * * *", minutes), nil

	default:
		return "", fmt.Errorf("不支持的调度类型: %s", scheduleType)
	}
}

// GetOnceTime 解析一次性任务的执行时间
func GetOnceTime(config string) (time.Time, error) {
	var cfg map[string]interface{}
	if err := json.Unmarshal([]byte(config), &cfg); err != nil {
		return time.Time{}, fmt.Errorf("解析调度配置失败: %w", err)
	}

	datetimeStr, _ := cfg["datetime"].(string)
	if datetimeStr == "" {
		return time.Time{}, fmt.Errorf("once类型需要datetime字段")
	}

	// 尝试多种时间格式
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.ParseInLocation(format, datetimeStr, time.Local); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("无法解析时间: %s", datetimeStr)
}

// ReplaceVariables 替换字符串中的变量 {{key}}
func ReplaceVariables(text string, variables map[string]interface{}) string {
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// 添加内置变量
	now := time.Now()
	variables["date"] = now.Format("2006-01-02")
	variables["time"] = now.Format("15:04:05")
	variables["datetime"] = now.Format("2006-01-02 15:04:05")
	variables["weekday"] = chineseWeekday(now.Weekday())
	variables["year"] = now.Format("2006")
	variables["month"] = now.Format("01")
	variables["day"] = now.Format("02")

	for k, v := range variables {
		placeholder := fmt.Sprintf("{{%s}}", k)
		text = strings.ReplaceAll(text, placeholder, fmt.Sprintf("%v", v))
	}

	return text
}

// parseTimeStr 解析 "HH:MM" 格式的时间
func parseTimeStr(s string) (int, int) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return 9, 0
	}
	var hour, minute int
	fmt.Sscanf(parts[0], "%d", &hour)
	fmt.Sscanf(parts[1], "%d", &minute)
	if hour < 0 || hour > 23 {
		hour = 9
	}
	if minute < 0 || minute > 59 {
		minute = 0
	}
	return hour, minute
}

// chineseWeekday 返回中文星期
func chineseWeekday(w time.Weekday) string {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(w)]
}

// ==================== 模板相关 ====================

// GetTaskTemplates 获取任务模板列表
func (s *AutomationService) GetTaskTemplates() ([]models.TaskTemplate, error) {
	query := `SELECT id, name, COALESCE(icon, ''), COALESCE(description, ''), task_type,
				default_config, default_schedule_type, default_schedule_config, steps, is_system,
				COALESCE(created_at, ''), COALESCE(updated_at, '')
			  FROM task_templates ORDER BY is_system DESC, id ASC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []models.TaskTemplate
	for rows.Next() {
		var t models.TaskTemplate
		if err := rows.Scan(&t.ID, &t.Name, &t.Icon, &t.Description, &t.TaskType,
			&t.DefaultConfig, &t.DefaultScheduleType, &t.DefaultScheduleConfig,
			&t.Steps, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt); err != nil {
			continue
		}
		templates = append(templates, t)
	}
	return templates, nil
}

// GetTaskTemplate 获取单个模板
func (s *AutomationService) GetTaskTemplate(id int) (*models.TaskTemplate, error) {
	query := `SELECT id, name, COALESCE(icon, ''), COALESCE(description, ''), task_type,
				default_config, default_schedule_type, default_schedule_config, steps, is_system,
				COALESCE(created_at, ''), COALESCE(updated_at, '')
			  FROM task_templates WHERE id = ?`

	row := s.db.QueryRow(query, id)
	var t models.TaskTemplate
	err := row.Scan(&t.ID, &t.Name, &t.Icon, &t.Description, &t.TaskType,
		&t.DefaultConfig, &t.DefaultScheduleType, &t.DefaultScheduleConfig,
		&t.Steps, &t.IsSystem, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("模板 %d 不存在", id)
		}
		return nil, err
	}
	return &t, nil
}

// CreateTaskFromTemplate 从模板创建任务
func (s *AutomationService) CreateTaskFromTemplate(templateID int, name, description string, scheduleType, scheduleConfig, slashCommand string) (int, error) {
	template, err := s.GetTaskTemplate(templateID)
	if err != nil {
		return 0, err
	}

	config := template.DefaultConfig
	if description == "" {
		description = template.Description
	}
	if scheduleType == "" {
		scheduleType = template.DefaultScheduleType
	}
	if scheduleConfig == "" {
		scheduleConfig = template.DefaultScheduleConfig
	}

	task := &models.AutomationTask{
		Name:             name,
		Description:      description,
		TaskType:         template.TaskType,
		Config:           config,
		ScheduleType:     scheduleType,
		ScheduleConfig:   scheduleConfig,
		Enabled:          true,
		MaxRetries:       2,
		RetryIntervalSec: 30,
		SlashCommand:     slashCommand,
	}

	taskID, err := s.CreateTask(task)
	if err != nil {
		return 0, err
	}

	if template.Steps != "" && template.Steps != "[]" {
		err = s.SaveStepsJSON(taskID, template.Steps)
		if err != nil {
			return taskID, err
		}
	}

	return taskID, nil
}

// GetTaskBySlashCommand 根据斜杠命令获取任务
func (s *AutomationService) GetTaskBySlashCommand(command string) (*models.AutomationTask, error) {
	query := `SELECT id, name, COALESCE(description, ''), task_type, config, schedule_type, schedule_config,
				enabled, COALESCE(status, ''), COALESCE(last_run_at, ''), COALESCE(next_run_at, ''),
				max_retries, retry_interval_sec, COALESCE(slash_command, ''), COALESCE(created_at, ''), COALESCE(updated_at, '')
			  FROM automation_tasks WHERE slash_command = ? AND enabled = 1`

	row := s.db.QueryRow(query, command)
	task, err := scanTaskRow(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("未找到命令 %s 对应的任务", command)
		}
		return nil, err
	}
	return &task, nil
}
