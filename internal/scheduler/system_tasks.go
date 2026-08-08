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
// WAL 模式下的 SQLite 数据库还包含 -wal / -shm 伴生文件，
// 只拷 .db 文件会丢失尚未 checkpoint 的事务，导致最近的数据丢失。
// 此处先做一次被动 checkpoint 让数据落盘，再同时备份三个文件。
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

	ts := time.Now().Format("20060102_150405")
	backupBase := fmt.Sprintf("along_backup_%s.db", ts)
	backupPath := filepath.Join(backupDir, backupBase)

	// 1) 先执行被动 checkpoint，把 -wal 中的已提交数据落盘到主库。
	//    checkpoint 是尽力而为的；若数据库当前被独占写占用，PASSIVE
	//    不会阻塞，仅记录当前可 checkpoint 的范围。
	if err := passiveCheckpoint(dbPath); err != nil {
		// 不直接失败——仍尝试做文件级拷贝，比完全无备份好。
		fmt.Printf("[backup] 被动 checkpoint 失败: %v（继续文件级备份）\n", err)
	}

	// 2) 同时备份主库 + WAL + SHM 三个文件
	srcFiles := []string{dbPath, dbPath + "-wal", dbPath + "-shm"}
	copied := 0
	for _, src := range srcFiles {
		if _, err := os.Stat(src); err != nil {
			// -wal / -shm 在 checkpoint 后可能已不存在/未创建，跳过
			continue
		}
		dst := backupPath
		if filepath.Ext(src) == "-wal" {
			dst = backupPath + "-wal"
		} else if filepath.Ext(src) == "-shm" {
			dst = backupPath + "-shm"
		}
		if err := copyFile(src, dst); err != nil {
			// 单个伴生文件失败不阻断主备份
			fmt.Printf("[backup] 复制 %s 失败: %v\n", src, err)
			continue
		}
		copied++
	}

	if copied == 0 {
		return models.TaskResult{Success: false, StatusText: "未复制任何文件"}
	}

	// 清理旧备份
	cleanOldBackups(backupDir, retentionCount)

	return models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("备份完成: %s（%d 个文件）", backupPath, copied),
		ResultType: "file",
		FilePath:   backupPath,
		Content:    fmt.Sprintf("数据库已备份到 %s（包含 %d 个文件）", backupPath, copied),
		Variables:  map[string]interface{}{"backup_path": backupPath},
	}
}

// passiveCheckpoint 打开数据库执行 PRAGMA wal_checkpoint(PASSIVE)
// 目的：把 -wal 中已提交事务合并到主库，再做文件级拷贝就能拿到一致快照。
// 注意：此函数必须通过 *sql.DB 连接执行；这里通过临时 sqlite 打开文件。
func passiveCheckpoint(dbPath string) error {
	// 用独立的 sqlite 连接做 checkpoint，不影响运行中的连接池
	dsn := dbPath + "?_busy_timeout=5000&_journal_mode=WAL"
	conn, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return err
	}
	defer conn.Close()
	_, err = conn.Exec("PRAGMA wal_checkpoint(PASSIVE)")
	return err
}

// copyFile 简单文件拷贝
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
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
