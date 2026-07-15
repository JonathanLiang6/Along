package scheduler

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// executeBackup 执行数据库备份
func executeBackup(configJSON string, dataDir string) models.TaskResult {
	cfg := parseConfig(configJSON)

	backupDir, _ := cfg["backup_dir"].(string)
	if backupDir == "" {
		backupDir = filepath.Join(dataDir, "backups")
	}

	retentionCount := 10
	if rc, ok := cfg["retention_count"].(float64); ok {
		retentionCount = int(rc)
	}

	dbPath, _ := cfg["db_path"].(string)
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "companion.db")
	}

	os.MkdirAll(backupDir, 0755)

	backupName := fmt.Sprintf("along_backup_%s.db", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(backupDir, backupName)

	src, err := os.Open(dbPath)
	if err != nil {
		return models.TaskResult{Success: false, StatusText: "打开数据库文件失败: " + err.Error()}
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return models.TaskResult{Success: false, StatusText: "创建备份文件失败: " + err.Error()}
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return models.TaskResult{Success: false, StatusText: "复制数据库失败: " + err.Error()}
	}

	// 清理旧备份
	cleanOldBackups(backupDir, retentionCount)

	return models.TaskResult{
		Success:    true,
		StatusText: "备份完成: " + backupPath,
		ResultType: "file",
		FilePath:   backupPath,
		Content:    fmt.Sprintf("数据库已备份到 %s", backupPath),
		Variables:  map[string]interface{}{"backup_path": backupPath},
	}
}

func cleanOldBackups(dir string, keep int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	var backups []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "along_backup_") && strings.HasSuffix(e.Name(), ".db") {
			backups = append(backups, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))
	for i := keep; i < len(backups); i++ {
		os.Remove(filepath.Join(dir, backups[i]))
	}
}

// executeCleanup 执行数据清理
func executeCleanup(configJSON string, db *sql.DB) models.TaskResult {
	cfg := parseConfig(configJSON)

	tableName, _ := cfg["table"].(string)
	if tableName == "" {
		return models.TaskResult{Success: false, StatusText: "未指定清理表"}
	}

	retentionDays := 30
	if rd, ok := cfg["retention_days"].(float64); ok {
		retentionDays = int(rd)
	}

	allowedTables := map[string]string{
		"automation_executions":      "started_at",
		"automation_step_executions": "id",
		"messages":                   "timestamp",
		"observations":               "created_at",
	}

	dateColumn, allowed := allowedTables[tableName]
	if !allowed {
		return models.TaskResult{
			Success:    false,
			StatusText: fmt.Sprintf("不允许清理表: %s", tableName),
		}
	}

	var query string
	if dateColumn == "id" {
		keepCount := 1000
		if kc, ok := cfg["keep_count"].(float64); ok {
			keepCount = int(kc)
		}
		query = fmt.Sprintf("DELETE FROM %s WHERE id NOT IN (SELECT id FROM %s ORDER BY id DESC LIMIT %d)", tableName, tableName, keepCount)
	} else {
		cutoff := time.Now().AddDate(0, 0, -retentionDays).Format("2006-01-02 15:04:05")
		query = fmt.Sprintf("DELETE FROM %s WHERE datetime(%s) < datetime('%s')", tableName, dateColumn, cutoff)
	}

	result, err := db.Exec(query)
	if err != nil {
		return models.TaskResult{Success: false, StatusText: "清理失败: " + err.Error()}
	}

	rowsAffected, _ := result.RowsAffected()
	return models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("已清理 %s 表 %d 条记录", tableName, rowsAffected),
		ResultType: "text",
		Content:    fmt.Sprintf("清理表: %s, 删除记录数: %d", tableName, rowsAffected),
		Variables:  map[string]interface{}{"rows_deleted": rowsAffected, "table": tableName},
	}
}

// 确保 services 被引用（parseConfig 使用了它）
var _ = services.ReplaceVariables
