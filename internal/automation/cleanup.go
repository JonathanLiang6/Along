package automation

import (
	"database/sql"
	"fmt"
	"time"

	"ai-companion/internal/models"
)

// CleanupExecutor 数据清理执行器
type CleanupExecutor struct {
	db *sql.DB
}

func (e *CleanupExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	tableName, _ := config["table"].(string)
	if tableName == "" {
		return &models.TaskResult{Success: false, StatusText: "未指定清理表", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	retentionDays := 30
	if rd, ok := config["retention_days"].(float64); ok {
		retentionDays = int(rd)
	}

	// 允许清理的表白名单
	allowedTables := map[string]string{
		"automation_executions":      "started_at",
		"automation_step_executions": "id",
		"messages":                   "timestamp",
		"observations":               "created_at",
	}

	dateColumn, allowed := allowedTables[tableName]
	if !allowed {
		return &models.TaskResult{
			Success:    false,
			StatusText: fmt.Sprintf("不允许清理表: %s", tableName),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 执行删除
	var query string
	if dateColumn == "id" {
		// 对于没有时间列的表，按ID保留最新的N条
		keepCount := 1000
		if kc, ok := config["keep_count"].(float64); ok {
			keepCount = int(kc)
		}
		query = fmt.Sprintf("DELETE FROM %s WHERE id NOT IN (SELECT id FROM %s ORDER BY id DESC LIMIT %d)", tableName, tableName, keepCount)
	} else {
		cutoffDate := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")
		query = fmt.Sprintf("DELETE FROM %s WHERE datetime(%s) < datetime('%s')", tableName, dateColumn, cutoffDate)
	}

	result, err := e.db.Exec(query)
	if err != nil {
		return &models.TaskResult{
			Success:    false,
			StatusText: "清理失败: " + err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	rowsAffected, _ := result.RowsAffected()

	taskResult := &models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("已清理 %s 表 %d 条记录", tableName, rowsAffected),
		ResultType: "text",
		Content:    fmt.Sprintf("清理表: %s, 删除记录数: %d, 保留天数: %d", tableName, rowsAffected, retentionDays),
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"rows_deleted": rowsAffected, "table": tableName},
	}

	return taskResult, nil
}

func (e *CleanupExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "table", Label: "清理表", Type: "select", Required: true, Options: []models.ConfigOption{
			{Value: "automation_executions", Label: "任务执行记录"},
			{Value: "automation_step_executions", Label: "步骤执行记录"},
			{Value: "messages", Label: "对话消息"},
			{Value: "observations", Label: "观察记录"},
		}},
		{Key: "retention_days", Label: "保留天数", Type: "number", Default: "30"},
		{Key: "keep_count", Label: "保留条数（无时间列时生效）", Type: "number", Default: "1000"},
	}
}
