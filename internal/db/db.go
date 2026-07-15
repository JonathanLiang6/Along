package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
)

// InitDB 初始化数据库并创建表
func InitDB(dbPath string) (*sql.DB, error) {
	// 添加 busy_timeout 和 WAL 模式，防止数据库锁问题
	db, err := sql.Open("sqlite3", dbPath+"?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on")
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
	if err := addColumnIfMissing(db, "automation_tasks", "slash_command", "TEXT DEFAULT ''"); err != nil {
		fmt.Println("添加 slash_command 字段失败:", err)
	}
	// 创建索引（CREATE INDEX IF NOT EXISTS 是安全的）
	if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_slash_cmd ON automation_tasks(slash_command) WHERE slash_command != ''`); err != nil {
		fmt.Println("创建 slash_command 索引失败:", err)
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
			slash_command TEXT DEFAULT '',
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
		// 14. task_templates 表（任务模板）
		`CREATE TABLE IF NOT EXISTS task_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			icon TEXT DEFAULT '',
			description TEXT,
			task_type TEXT NOT NULL DEFAULT 'workflow',
			default_config TEXT NOT NULL DEFAULT '{}',
			default_schedule_type TEXT DEFAULT 'weekly',
			default_schedule_config TEXT NOT NULL DEFAULT '{}',
			steps TEXT NOT NULL DEFAULT '[]',
			is_system BOOLEAN DEFAULT 1,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE INDEX IF NOT EXISTS idx_templates_system ON task_templates(is_system);`,
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
	var taskCount int
	db.QueryRow("SELECT count(*) FROM automation_tasks").Scan(&taskCount)
	if taskCount > 0 {
		return nil
	}

	schedulesTable := ""
	toolflowsTable := ""
	scheduleExecTable := ""
	toolflowExecTable := ""

	checkTable := func(orig, legacy string) string {
		var cnt int
		db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", orig).Scan(&cnt)
		if cnt > 0 {
			return orig
		}
		db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", legacy).Scan(&cnt)
		if cnt > 0 {
			return legacy
		}
		return ""
	}

	schedulesTable = checkTable("schedules", "schedules_legacy")
	toolflowsTable = checkTable("toolflows", "toolflows_legacy")
	scheduleExecTable = checkTable("schedule_executions", "schedule_executions_legacy")
	toolflowExecTable = checkTable("toolflow_executions", "toolflow_executions_legacy")

	if schedulesTable == "" && toolflowsTable == "" {
		return nil
	}

	log.Println("开始迁移旧表数据到自动化任务系统...")

	scheduleIDMap := make(map[int]int)
	toolflowIDMap := make(map[int]int)

	type toolflowRow struct {
		id          int
		name        string
		description string
		steps       string
		enabled     bool
		createdAt   sql.NullString
		updatedAt   sql.NullString
	}
	var toolflowRows []toolflowRow
	if toolflowsTable != "" {
		rows, err := db.Query(fmt.Sprintf("SELECT id, name, description, steps, enabled, created_at, updated_at FROM %s", toolflowsTable))
		if err == nil {
			for rows.Next() {
				var r toolflowRow
				if rows.Scan(&r.id, &r.name, &r.description, &r.steps, &r.enabled, &r.createdAt, &r.updatedAt) != nil {
					continue
				}
				toolflowRows = append(toolflowRows, r)
			}
			rows.Close()
		}
		for _, r := range toolflowRows {
			result, err := db.Exec(`INSERT INTO automation_tasks (name, description, task_type, config, schedule_type, schedule_config, enabled, status, created_at, updated_at) VALUES (?, ?, 'workflow', '{}', 'custom', '{}', ?, 'idle', ?, ?)`, r.name, r.description, r.enabled, r.createdAt.String, r.updatedAt.String)
			if err != nil {
				continue
			}
			newID, _ := result.LastInsertId()
			toolflowIDMap[r.id] = int(newID)
			migrateToolFlowSteps(db, int(newID), r.steps)
		}
	}

	type scheduleRow struct {
		id            int
		name          string
		cronExpr      string
		description   string
		actionType    string
		actionPayload string
		enabled       bool
		lastRunAt     sql.NullString
		nextRunAt     sql.NullString
		status        sql.NullString
		createdAt     sql.NullString
		updatedAt     sql.NullString
	}
	var scheduleRows []scheduleRow
	if schedulesTable != "" {
		rows, err := db.Query(fmt.Sprintf("SELECT id, name, cron_expression, description, action_type, action_payload, enabled, last_run_at, next_run_at, status, created_at, updated_at FROM %s", schedulesTable))
		if err == nil {
			for rows.Next() {
				var r scheduleRow
				if rows.Scan(&r.id, &r.name, &r.cronExpr, &r.description, &r.actionType, &r.actionPayload, &r.enabled, &r.lastRunAt, &r.nextRunAt, &r.status, &r.createdAt, &r.updatedAt) != nil {
					continue
				}
				scheduleRows = append(scheduleRows, r)
			}
			rows.Close()
		}
		for _, r := range scheduleRows {
			taskType := "agent_chat"
			config := "{}"
			switch r.actionType {
			case "agent_call":
				taskType = "agent_chat"
				config = r.actionPayload
			case "message":
				taskType = "reminder"
				config = r.actionPayload
			case "tool_flow":
				taskType = "workflow"
				config = r.actionPayload
			}

			scheduleConfig := fmt.Sprintf(`{"type":"custom","cron":"%s"}`, r.cronExpr)

			statusVal := "idle"
			if r.status.Valid {
				switch r.status.String {
				case "success", "failed", "running":
					statusVal = r.status.String
				}
			}

			result, err := db.Exec(`INSERT INTO automation_tasks (name, description, task_type, config, schedule_type, schedule_config, enabled, status, last_run_at, next_run_at, created_at, updated_at) VALUES (?, ?, ?, ?, 'custom', ?, ?, ?, ?, ?, ?, ?)`, r.name, r.description, taskType, config, scheduleConfig, r.enabled, statusVal, r.lastRunAt.String, r.nextRunAt.String, r.createdAt.String, r.updatedAt.String)
			if err != nil {
				continue
			}
			newID, _ := result.LastInsertId()
			scheduleIDMap[r.id] = int(newID)
		}
	}

	type execRow struct {
		id           int
		refID        int
		status       sql.NullString
		errorMessage sql.NullString
		result       sql.NullString
		executedAt   sql.NullString
	}
	var scheduleExecRows []execRow
	if scheduleExecTable != "" {
		rows, err := db.Query(fmt.Sprintf("SELECT id, schedule_id, status, error_message, result, executed_at FROM %s", scheduleExecTable))
		if err == nil {
			for rows.Next() {
				var r execRow
				if rows.Scan(&r.id, &r.refID, &r.status, &r.errorMessage, &r.result, &r.executedAt) != nil {
					continue
				}
				scheduleExecRows = append(scheduleExecRows, r)
			}
			rows.Close()
		}
		for _, r := range scheduleExecRows {
			newTaskID, ok := scheduleIDMap[r.refID]
			if !ok {
				continue
			}
			db.Exec(`INSERT INTO automation_executions (task_id, status, result_type, result_content, error_message, started_at, finished_at) VALUES (?, ?, 'text', ?, ?, ?, ?)`, newTaskID, r.status.String, r.result.String, r.errorMessage.String, r.executedAt.String, r.executedAt.String)
		}
	}

	var toolflowExecRows []struct {
		id           int
		refID        int
		status       sql.NullString
		inputs       sql.NullString
		outputs      sql.NullString
		errorMessage sql.NullString
		startedAt    sql.NullString
		finishedAt   sql.NullString
	}
	if toolflowExecTable != "" {
		rows, err := db.Query(fmt.Sprintf("SELECT id, toolflow_id, status, inputs, outputs, error_message, started_at, finished_at FROM %s", toolflowExecTable))
		if err == nil {
			for rows.Next() {
				var r struct {
					id           int
					refID        int
					status       sql.NullString
					inputs       sql.NullString
					outputs      sql.NullString
					errorMessage sql.NullString
					startedAt    sql.NullString
					finishedAt   sql.NullString
				}
				if rows.Scan(&r.id, &r.refID, &r.status, &r.inputs, &r.outputs, &r.errorMessage, &r.startedAt, &r.finishedAt) != nil {
					continue
				}
				toolflowExecRows = append(toolflowExecRows, r)
			}
			rows.Close()
		}
		for _, r := range toolflowExecRows {
			newTaskID, ok := toolflowIDMap[r.refID]
			if !ok {
				continue
			}
			db.Exec(`INSERT INTO automation_executions (task_id, status, result_type, result_content, error_message, started_at, finished_at) VALUES (?, ?, 'text', ?, ?, ?, ?)`, newTaskID, r.status.String, r.outputs.String, r.errorMessage.String, r.startedAt.String, r.finishedAt.String)
		}
	}

	for _, table := range []string{"schedules", "schedule_executions", "toolflows", "toolflow_executions"} {
		var cnt int
		db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&cnt)
		if cnt > 0 {
			db.Exec(fmt.Sprintf("ALTER TABLE %s RENAME TO %s_legacy", table, table))
		}
	}

	log.Println("旧表数据迁移完成")
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
