package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// InitDB 初始化数据库并创建表
func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	// 创建表
	if err := createTables(db); err != nil {
		return nil, fmt.Errorf("创建表失败: %w", err)
	}

	// 尝试添加新字段（兼容旧库）
	if err := addColumnIfMissing(db, "conversations", "agent_route", "TEXT"); err != nil {
		fmt.Println("添加 agent_route 字段失败:", err)
	}
	if err := addColumnIfMissing(db, "memories", "tags", "TEXT"); err != nil {
		fmt.Println("添加 tags 字段失败:", err)
	}
	if err := addColumnIfMissing(db, "reflections", "observations", "TEXT"); err != nil {
		fmt.Println("添加 observations 字段失败:", err)
	}

	// 迁移旧表数据到新的自动化任务系统
	if err := migrateLegacyTables(db); err != nil {
		fmt.Println("迁移旧表数据失败:", err)
	}

	return db, nil
}

func createTables(db *sql.DB) error {
	queries := []string{
		// 1. 记忆表（5层记忆）
		`CREATE TABLE IF NOT EXISTS memories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			type TEXT NOT NULL,
			content TEXT NOT NULL,
			source TEXT,
			confidence REAL DEFAULT 0.5,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			tags TEXT
		);`,
		// 2. 对话表
		`CREATE TABLE IF NOT EXISTS conversations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date DATE NOT NULL,
			title TEXT,
			agent_route TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 3. 消息表
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT NOT NULL,
			emotion TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (conversation_id) REFERENCES conversations(id)
		);`,
		// 4. 计划表（学习/项目/习惯/生活）
		`CREATE TABLE IF NOT EXISTS goals (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			type TEXT NOT NULL DEFAULT 'project',
			status TEXT NOT NULL DEFAULT 'active',
			start_date TEXT,
			target_date TEXT,
			current_focus TEXT,
			next_step TEXT,
			companion_note TEXT,
			progress INTEGER DEFAULT 0,
			mood TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 5. 里程碑表
		`CREATE TABLE IF NOT EXISTS milestones (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			description TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			completed_at TEXT,
			companion_comment TEXT,
			order_index INTEGER DEFAULT 0,
			FOREIGN KEY (goal_id) REFERENCES goals(id) ON DELETE CASCADE
		);`,
		// 6. 计划表-记录（日记式打卡）
		`CREATE TABLE IF NOT EXISTS check_ins (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			goal_id INTEGER NOT NULL,
			date TEXT NOT NULL,
			content TEXT NOT NULL,
			mood TEXT,
			companion_response TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (goal_id) REFERENCES goals(id) ON DELETE CASCADE
		);`,
		// 5. 观察表
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			content TEXT NOT NULL,
			type TEXT NOT NULL,
			displayed BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 6. 高光回忆表
		`CREATE TABLE IF NOT EXISTS highlights (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			description TEXT,
			date DATE,
			memory_ids TEXT,
			user_marked BOOLEAN DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 7. 复盘表
		`CREATE TABLE IF NOT EXISTS reflections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			period_start DATE,
			period_end DATE,
			growth_analysis TEXT,
			relationship_analysis TEXT,
			project_review TEXT,
			observations TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 8. 设置表
		`CREATE TABLE IF NOT EXISTS settings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT UNIQUE NOT NULL,
			value TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 9. automation_tasks 表（替代 schedules + toolflows）
		`CREATE TABLE IF NOT EXISTS automation_tasks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			task_type TEXT NOT NULL,
			config TEXT NOT NULL DEFAULT '{}',
			schedule_type TEXT NOT NULL DEFAULT 'daily',
			schedule_config TEXT NOT NULL DEFAULT '{}',
			enabled BOOLEAN DEFAULT 1,
			status TEXT DEFAULT 'idle',
			last_run_at DATETIME,
			next_run_at DATETIME,
			max_retries INTEGER DEFAULT 2,
			retry_interval_sec INTEGER DEFAULT 30,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		// 10. automation_steps 表（workflow类型专用）
		`CREATE TABLE IF NOT EXISTS automation_steps (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL,
			step_index INTEGER NOT NULL,
			step_type TEXT NOT NULL,
			name TEXT NOT NULL DEFAULT '',
			config TEXT NOT NULL DEFAULT '{}',
			output_var TEXT NOT NULL DEFAULT '',
			next_on_success INTEGER DEFAULT 0,
			next_on_failure INTEGER DEFAULT -1,
			FOREIGN KEY (task_id) REFERENCES automation_tasks(id) ON DELETE CASCADE
		);`,
		// 11. automation_dependencies 表（任务依赖）
		`CREATE TABLE IF NOT EXISTS automation_dependencies (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL,
			depends_on_id INTEGER NOT NULL,
			condition TEXT NOT NULL DEFAULT 'on_success',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (task_id) REFERENCES automation_tasks(id) ON DELETE CASCADE,
			FOREIGN KEY (depends_on_id) REFERENCES automation_tasks(id) ON DELETE CASCADE
		);`,
		// 12. automation_executions 表（替代 schedule_executions + toolflow_executions）
		`CREATE TABLE IF NOT EXISTS automation_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'running',
			result_type TEXT DEFAULT 'none',
			result_content TEXT,
			result_path TEXT,
			error_message TEXT,
			retry_count INTEGER DEFAULT 0,
			duration_ms INTEGER DEFAULT 0,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			finished_at DATETIME,
			FOREIGN KEY (task_id) REFERENCES automation_tasks(id) ON DELETE CASCADE
		);`,
		// 13. automation_step_executions 表（步骤级进度）
		`CREATE TABLE IF NOT EXISTS automation_step_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			execution_id INTEGER NOT NULL,
			step_index INTEGER NOT NULL,
			step_name TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'pending',
			input_preview TEXT,
			output_preview TEXT,
			duration_ms INTEGER DEFAULT 0,
			error_message TEXT,
			FOREIGN KEY (execution_id) REFERENCES automation_executions(id) ON DELETE CASCADE
		);`,
		// 索引
		`CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);`,
		`CREATE INDEX IF NOT EXISTS idx_observations_type ON observations(type);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_timestamp ON messages(timestamp);`,
		`CREATE INDEX IF NOT EXISTS idx_conversations_date ON conversations(date);`,
		`CREATE INDEX IF NOT EXISTS idx_memories_confidence ON memories(confidence);`,
		`CREATE INDEX IF NOT EXISTS idx_goals_type ON goals(type);`,
		`CREATE INDEX IF NOT EXISTS idx_goals_status ON goals(status);`,
		`CREATE INDEX IF NOT EXISTS idx_milestones_goal ON milestones(goal_id);`,
		`CREATE INDEX IF NOT EXISTS idx_check_ins_goal ON check_ins(goal_id);`,
		`CREATE INDEX IF NOT EXISTS idx_check_ins_date ON check_ins(date);`,
		`CREATE INDEX IF NOT EXISTS idx_automation_tasks_type ON automation_tasks(task_type);`,
		`CREATE INDEX IF NOT EXISTS idx_automation_tasks_enabled ON automation_tasks(enabled);`,
		`CREATE INDEX IF NOT EXISTS idx_automation_steps_task ON automation_steps(task_id);`,
		`CREATE INDEX IF NOT EXISTS idx_automation_executions_task ON automation_executions(task_id);`,
		`CREATE INDEX IF NOT EXISTS idx_automation_step_executions_exec ON automation_step_executions(execution_id);`,
	}

	for _, query := range queries {
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("执行SQL失败: %w", err)
		}
	}

	return nil
}

// addColumnIfMissing 兼容旧库：仅在字段不存在时添加
func addColumnIfMissing(db *sql.DB, table, column, defType string) error {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			continue
		}
		if name == column {
			return nil // 已存在
		}
	}
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, defType))
	return err
}

// migrateLegacyTables 迁移旧表数据到新的自动化表
func migrateLegacyTables(db *sql.DB) error {
	// 检查旧表是否存在
	var count int
	err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name='schedules'").Scan(&count)
	if err != nil || count == 0 {
		return nil
	}

	// 检查是否已迁移（automation_tasks 是否有数据）
	var taskCount int
	db.QueryRow("SELECT count(*) FROM automation_tasks").Scan(&taskCount)
	if taskCount > 0 {
		return nil // 已迁移
	}

	fmt.Println("开始迁移旧表数据到自动化任务系统...")

	// 1. 迁移 toolflows -> automation_tasks (task_type='workflow')
	rows, err := db.Query("SELECT id, name, description, steps, enabled, created_at, updated_at FROM toolflows")
	if err != nil {
		return err
	}

	toolflowIDMap := make(map[int]int) // 旧ID -> 新task ID
	for rows.Next() {
		var id int
		var name, description, steps string
		var enabled bool
		var createdAt, updatedAt string
		if err := rows.Scan(&id, &name, &description, &steps, &enabled, &createdAt, &updatedAt); err != nil {
			continue
		}

		result, err := db.Exec(`INSERT INTO automation_tasks (name, description, task_type, config, schedule_type, schedule_config, enabled, status, created_at, updated_at)
			VALUES (?, ?, 'workflow', '{}', 'custom', '{}', ?, 'idle', ?, ?)`,
			name, description, enabled, createdAt, updatedAt)
		if err != nil {
			continue
		}
		newID, _ := result.LastInsertId()
		toolflowIDMap[id] = int(newID)

		// 迁移 steps JSON 到 automation_steps 表
		migrateToolFlowSteps(db, int(newID), steps)
	}
	rows.Close()

	// 2. 迁移 schedules -> automation_tasks
	rows, err = db.Query("SELECT id, name, cron_expression, description, action_type, action_payload, enabled, last_run_at, next_run_at, status, created_at, updated_at FROM schedules")
	if err != nil {
		log.Printf("查询schedules表失败: %v", err)
	} else {
		scheduleIDMap := make(map[int]int)
		for rows.Next() {
			var id int
			var name, cronExpr, description, actionType, actionPayload string
			var enabled bool
			var lastRunAt, nextRunAt, status, createdAt, updatedAt sql.NullString

			if err := rows.Scan(&id, &name, &cronExpr, &description, &actionType, &actionPayload, &enabled, &lastRunAt, &nextRunAt, &status, &createdAt, &updatedAt); err != nil {
				log.Printf("扫描schedule行失败: %v", err)
				continue
			}

			// 转换 action_type -> task_type
			taskType := "agent_chat"
			config := "{}"
			switch actionType {
			case "agent_call":
				taskType = "agent_chat"
				config = actionPayload
			case "message":
				taskType = "reminder"
				config = actionPayload
			case "tool_flow":
				taskType = "workflow"
				config = actionPayload
			}

			scheduleConfig := fmt.Sprintf(`{"type":"custom","cron":"%s"}`, cronExpr)

			// 处理status映射
			statusVal := "idle"
			if status.Valid {
				switch status.String {
				case "success":
					statusVal = "success"
				case "failed":
					statusVal = "failed"
				case "running":
					statusVal = "running"
				}
			}

			result, err := db.Exec(`INSERT INTO automation_tasks (name, description, task_type, config, schedule_type, schedule_config, enabled, status, last_run_at, next_run_at, created_at, updated_at)
				VALUES (?, ?, ?, ?, 'custom', ?, ?, ?, ?, ?, ?, ?)`,
				name, description, taskType, config, scheduleConfig, enabled, statusVal,
				lastRunAt.String, nextRunAt.String,
				createdAt.String, updatedAt.String)
			if err != nil {
				log.Printf("插入automation_tasks失败: %v", err)
				continue
			}
			newID, _ := result.LastInsertId()
			scheduleIDMap[id] = int(newID)
		}
		rows.Close()

	// 3. 迁移 schedule_executions -> automation_executions
	rows, err = db.Query("SELECT id, schedule_id, status, error_message, result, executed_at FROM schedule_executions")
	if err == nil {
		for rows.Next() {
			var id, scheduleID int
			var status, errorMessage, result, executedAt string
			if err := rows.Scan(&id, &scheduleID, &status, &errorMessage, &result, &executedAt); err != nil {
				continue
			}
			newTaskID, ok := scheduleIDMap[scheduleID]
			if !ok {
				continue
			}
			db.Exec(`INSERT INTO automation_executions (task_id, status, result_type, result_content, error_message, started_at, finished_at)
				VALUES (?, ?, 'text', ?, ?, ?, ?)`,
				newTaskID, status, result, errorMessage, executedAt, executedAt)
		}
		rows.Close()
	}

	// 4. 迁移 toolflow_executions -> automation_executions
	rows, err = db.Query("SELECT id, toolflow_id, status, inputs, outputs, error_message, started_at, finished_at FROM toolflow_executions")
	if err == nil {
		for rows.Next() {
			var id, toolflowID int
			var status, inputs, outputs, errorMessage, startedAt, finishedAt string
			if err := rows.Scan(&id, &toolflowID, &status, &inputs, &outputs, &errorMessage, &startedAt, &finishedAt); err != nil {
				continue
			}
			newTaskID, ok := toolflowIDMap[toolflowID]
			if !ok {
				continue
			}
			db.Exec(`INSERT INTO automation_executions (task_id, status, result_type, result_content, error_message, started_at, finished_at)
				VALUES (?, ?, 'text', ?, ?, ?, ?)`,
				newTaskID, status, outputs, errorMessage, startedAt, finishedAt)
		}
		rows.Close()
	}

	// 5. 重命名旧表为 _legacy 备份
	db.Exec("ALTER TABLE schedules RENAME TO schedules_legacy")
	db.Exec("ALTER TABLE schedule_executions RENAME TO schedule_executions_legacy")
	db.Exec("ALTER TABLE toolflows RENAME TO toolflows_legacy")
	db.Exec("ALTER TABLE toolflow_executions RENAME TO toolflow_executions_legacy")

	fmt.Println("旧表数据迁移完成，旧表已重命名为 _legacy 后缀")
	return nil
}

// migrateToolFlowSteps 将工具流步骤JSON迁移到automation_steps表
func migrateToolFlowSteps(db *sql.DB, taskID int, stepsJSON string) {
	var steps []struct {
		ID         string                 `json:"id"`
		Type       string                 `json:"type"`
		AgentName  string                 `json:"agent_name,omitempty"`
		ToolAction string                 `json:"tool_action,omitempty"`
		Parameters map[string]interface{} `json:"parameters"`
		NextStep   string                 `json:"next_step,omitempty"`
		OnSuccess  string                 `json:"on_success,omitempty"`
		OnFailure  string                 `json:"on_failure,omitempty"`
	}

	if err := json.Unmarshal([]byte(stepsJSON), &steps); err != nil {
		return
	}

	for i, step := range steps {
		configBytes, _ := json.Marshal(step.Parameters)
		name := step.Type
		if step.AgentName != "" {
			name = step.AgentName
		} else if step.ToolAction != "" {
			name = step.ToolAction
		}

		db.Exec(`INSERT INTO automation_steps (task_id, step_index, step_type, name, config, output_var, next_on_success, next_on_failure)
			VALUES (?, ?, ?, ?, ?, '', 0, -1)`,
			taskID, i, step.Type, name, string(configBytes))
	}
}
