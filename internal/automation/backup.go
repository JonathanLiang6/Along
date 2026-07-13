package automation

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

// BackupExecutor 数据备份执行器
type BackupExecutor struct {
	db      *sql.DB
	dataDir string
}

func (e *BackupExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	backupDir, _ := config["backup_dir"].(string)
	if backupDir == "" {
		backupDir = filepath.Join(e.dataDir, "backups")
	}
	backupDir = services.ReplaceVariables(backupDir, ctx.Variables)

	retentionCount := 10
	if rc, ok := config["retention_count"].(float64); ok {
		retentionCount = int(rc)
	}

	// 获取数据库文件路径
	// 通过db对象无法直接获取文件路径，从配置中读取或使用dataDir下的默认路径
	dbPath, _ := config["db_path"].(string)
	if dbPath == "" {
		dbPath = filepath.Join(e.dataDir, "along.db")
	}

	// 确保备份目录存在
	os.MkdirAll(backupDir, 0755)

	// 生成备份文件名
	backupName := fmt.Sprintf("along_backup_%s.db", time.Now().Format("20060102_150405"))
	backupPath := filepath.Join(backupDir, backupName)

	// 复制数据库文件
	src, err := os.Open(dbPath)
	if err != nil {
		return &models.TaskResult{
			Success:    false,
			StatusText: "打开数据库文件失败: " + err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return &models.TaskResult{
			Success:    false,
			StatusText: "创建备份文件失败: " + err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return &models.TaskResult{
			Success:    false,
			StatusText: "复制数据库失败: " + err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	result := &models.TaskResult{
		Success:    true,
		StatusText: "备份完成: " + backupPath,
		ResultType: "file",
		FilePath:   backupPath,
		Content:    fmt.Sprintf("数据库已备份到 %s", backupPath),
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"backup_path": backupPath},
	}

	// 清理旧备份
	if retentionCount > 0 {
		e.cleanOldBackups(backupDir, retentionCount)
	}

	return result, nil
}

// cleanOldBackups 清理旧备份文件，保留最新的retentionCount个
func (e *BackupExecutor) cleanOldBackups(backupDir string, retentionCount int) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return
	}

	var backups []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "along_backup_") && strings.HasSuffix(entry.Name(), ".db") {
			backups = append(backups, entry.Name())
		}
	}

	// 按文件名排序（文件名包含时间戳，排序即按时间）
	sort.Sort(sort.Reverse(sort.StringSlice(backups)))

	// 删除超出保留数量的旧备份
	for i := retentionCount; i < len(backups); i++ {
		oldPath := filepath.Join(backupDir, backups[i])
		os.Remove(oldPath)
	}
}

func (e *BackupExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "backup_dir", Label: "备份目录", Type: "text", Placeholder: "留空则使用 dataDir/backups"},
		{Key: "db_path", Label: "数据库路径", Type: "text", Placeholder: "留空则使用 dataDir/along.db"},
		{Key: "retention_count", Label: "保留备份数量", Type: "number", Default: "10"},
	}
}
