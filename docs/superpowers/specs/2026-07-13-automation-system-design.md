# 统一自动化系统设计文档

> 日期: 2026-07-13
> 项目: Along - 多Agent助手
> 范围: 合并Scheduler和ToolFlow为统一自动化任务系统

---

## 一、背景与问题

### 现状

当前系统存在两套独立的自动化机制：

1. **Scheduler（周期任务）**：基于cron表达式定时触发，支持3种action_type（agent_call/tool_flow/message），用户需手写JSON配置
2. **ToolFlow（工具流）**：多步骤串联执行，支持5种step_type，步骤用JSON存储，前端编辑器简陋

### 核心问题

- **用户不友好**：需要手写Cron表达式和JSON配置，门槛极高
- **任务类型不合理**：agent_call/tool_flow/message三种类型不清晰，不够灵活
- **系统割裂**：任务和工作流是两个独立页面、两套数据模型、两套执行逻辑
- **执行逻辑简陋**：条件判断永远返回true，循环逻辑不完整，无失败重试
- **执行反馈不足**：只记录成功/失败，无步骤级进度、无结果预览、无文件路径展示
- **缺乏任务协作**：无依赖关系、无条件触发、无并行执行
- **产品定位偏移**：现有代码包含"江璐姐姐"人设，需改为多Agent助手along

---

## 二、设计目标

1. **统一概念**：合并Scheduler和ToolFlow为"自动化任务"
2. **用户友好**：填空式配置，不写JSON，不用Cron
3. **类型丰富**：10种预定义任务类型覆盖常见场景
4. **执行智能**：任务依赖、条件触发、失败重试、步骤级进度
5. **结果可视**：执行进度实时展示、结果预览、文件路径可打开
6. **品牌统一**：移除江璐姐姐人设，改为along多Agent助手

---

## 三、数据模型

### 3.1 automation_tasks 表（替代 schedules + toolflows）

```sql
CREATE TABLE IF NOT EXISTS automation_tasks (
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
);
```

**task_type 枚举值：**
- `agent_chat` - Agent对话
- `web_search` - 信息检索
- `report` - 报告生成
- `backup` - 数据备份
- `reminder` - 提醒通知
- `monitor` - 数据监控
- `habit_checkin` - 习惯打卡
- `review` - 知识复习
- `cleanup` - 清理维护
- `workflow` - 流程编排

**schedule_type 枚举值：** `once` / `daily` / `weekly` / `monthly` / `custom`

**schedule_config 示例：**

```json
{"type":"daily","hour":8,"min":0}
{"type":"weekly","days":[1,3,5],"hour":9,"min":0}
{"type":"monthly","day":1,"hour":10,"min":0}
{"type":"once","datetime":"2026-08-01T14:00"}
{"type":"custom","cron":"0 8 * * 1-5"}
```

**config 示例（web_search类型）：**

```json
{
    "query": "Go语言 最新动态",
    "engine": "duckduckgo",
    "result_count": 5,
    "need_summary": true,
    "output_type": "file",
    "file_path": "D:\\Reports\\tech_{{date}}.md"
}
```

### 3.2 automation_steps 表（workflow类型专用）

```sql
CREATE TABLE IF NOT EXISTS automation_steps (
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
);
```

**step_type 枚举值：** `agent` / `search` / `condition` / `save_file` / `notify`

**next_on_success / next_on_failure 含义：**
- `0` = 执行下一步
- `-1` = 结束
- `-2` = 重试本步
- `>0` = 跳转到指定step_index

### 3.3 automation_dependencies 表（任务依赖）

```sql
CREATE TABLE IF NOT EXISTS automation_dependencies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    task_id INTEGER NOT NULL,
    depends_on_id INTEGER NOT NULL,
    condition TEXT NOT NULL DEFAULT 'on_success',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (task_id) REFERENCES automation_tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (depends_on_id) REFERENCES automation_tasks(id) ON DELETE CASCADE
);
```

**condition 枚举值：** `on_success` / `on_failure` / `on_complete`

### 3.4 automation_executions 表（替代 schedule_executions + toolflow_executions）

```sql
CREATE TABLE IF NOT EXISTS automation_executions (
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
);
```

**result_type 枚举值：** `text` / `file` / `none`

### 3.5 automation_step_executions 表（步骤级进度）

```sql
CREATE TABLE IF NOT EXISTS automation_step_executions (
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
);
```

---

## 四、任务类型配置Schema

每种任务类型有专属的配置字段定义，前端据此动态渲染表单。

### 4.1 agent_chat - Agent对话

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| agent_name | select | 是 | 可选: web/planner/emotion/memory/reflection/summarize/tool |
| prompt | textarea | 是 | 提示词，支持 {{date}} {{time}} {{weekday}} 变量 |
| output_type | select | 是 | record/notify/file |
| file_path | text | 条件 | output_type=file 时必填 |

### 4.2 web_search - 信息检索

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| query | text | 是 | 搜索关键词，支持变量 |
| engine | select | 否 | duckduckgo(默认)/bing |
| result_count | number | 否 | 1-10，默认5 |
| need_summary | boolean | 否 | 默认true |
| output_type | select | 是 | record/notify/file |
| file_path | text | 条件 | output_type=file 时必填 |

### 4.3 report - 报告生成

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| report_type | select | 是 | daily/weekly/monthly/custom |
| start_date | date | 条件 | report_type=custom 时必填 |
| end_date | date | 条件 | report_type=custom 时必填 |
| include | multi_select | 否 | learning/projects/emotion/memory |
| format | select | 否 | markdown(默认)/text |
| file_path | text | 是 | 保存路径，支持 {{date}} {{type}} 变量 |

### 4.4 backup - 数据备份

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| backup_content | multi_select | 是 | conversations/memories/tasks/all |
| backup_path | text | 是 | 保存目录 |
| file_template | text | 否 | 文件名模板，默认 along_backup_{{date}}.db |
| keep_count | number | 否 | 保留备份数，默认5 |

### 4.5 reminder - 提醒通知

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| content | textarea | 是 | 提醒内容 |
| repeat | boolean | 否 | 默认false |
| repeat_interval_min | number | 条件 | repeat=true 时必填，默认30 |

### 4.6 monitor - 数据监控

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| monitor_type | select | 是 | webpage/rss/file |
| url | text | 是 | 监控地址或文件路径 |
| check_frequency | select | 否 | 1h/6h/daily，默认daily |
| action_on_change | select | 否 | notify/notify_summarize，默认notify |

### 4.7 habit_checkin - 习惯打卡

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| habit_name | text | 是 | 习惯名称 |
| reminder_text | textarea | 是 | 提醒内容 |
| reminder_time | time | 是 | 提醒时间 |
| stats_period | select | 否 | weekly/monthly，默认weekly |

### 4.8 review - 知识复习

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| topic | text | 是 | 知识主题 |
| material | textarea | 否 | 复习内容或文件路径 |
| plan | select | 是 | ebbinghaus/custom |
| custom_intervals | text | 条件 | plan=custom 时，逗号分隔天数 |
| start_date | date | 是 | 起始日期 |

### 4.9 cleanup - 清理维护

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| clean_type | multi_select | 是 | old_executions/old_conversations/temp_files |
| keep_days | number | 否 | 保留天数，默认90 |
| auto_execute | boolean | 否 | 默认false(仅提示不删除) |

### 4.10 workflow - 流程编排

workflow类型不使用config字段，而是使用automation_steps表存储步骤。配置详见第五部分。

---

## 五、流程编排设计

### 5.1 步骤类型

#### agent - Agent调用

| 配置项 | 类型 | 说明 |
|--------|------|------|
| agent_name | select | 选择Agent |
| prompt | textarea | 提示词，支持 {{变量名}} |
| output_var | text | 输出变量名 |

#### search - 网络搜索

| 配置项 | 类型 | 说明 |
|--------|------|------|
| query | text | 搜索关键词，支持 {{变量名}} |
| need_summary | boolean | 是否AI总结 |
| output_var | text | 输出变量名 |

#### condition - 条件判断

| 配置项 | 类型 | 说明 |
|--------|------|------|
| source_var | select | 选择前序步骤变量 |
| operator | select | contains/not_contains/equals/is_empty/not_empty |
| compare_value | text | 比较值 |
| next_on_success | number | 成功跳转(0=下一步,-1=结束) |
| next_on_failure | number | 失败跳转(0=下一步,-1=结束,-2=重试) |

#### save_file - 保存文件

| 配置项 | 类型 | 说明 |
|--------|------|------|
| content_var | select | 内容来源变量 |
| format | select | markdown/text/json |
| file_path | text | 目录路径 |
| file_name | text | 文件名，支持 {{date}} {{变量名}} |

#### notify - 发送通知

| 配置项 | 类型 | 说明 |
|--------|------|------|
| content | textarea | 通知内容，支持 {{变量名}} |
| level | select | normal/important |

### 5.2 变量传递机制

- 每个步骤通过 `output_var` 定义输出变量名
- 后续步骤在文本配置项中使用 `{{变量名}}` 引用前序步骤输出
- 前端提供"插入变量"按钮，列出所有前序步骤的output_var供选择
- 执行时引擎替换所有 `{{变量名}}` 为实际值

---

## 六、执行引擎

### 6.1 架构

```
Engine (统一调度)
  ├── Cron调度器 (robfig/cron)
  ├── 依赖触发器
  ├── TaskExecutorRegistry
  │     ├── AgentChatExecutor
  │     ├── WebSearchExecutor
  │     ├── ReportExecutor
  │     ├── BackupExecutor
  │     ├── ReminderExecutor
  │     ├── MonitorExecutor
  │     ├── HabitCheckinExecutor
  │     ├── ReviewExecutor
  │     ├── CleanupExecutor
  │     └── WorkflowExecutor
  └── 执行记录管理
```

### 6.2 接口定义

```go
type TaskExecutor interface {
    Execute(config map[string]interface{}, ctx TaskContext) (*TaskResult, error)
    ConfigSchema() []ConfigField
}

type TaskContext struct {
    TaskID    int
    TaskName  string
    TaskType  string
    Variables map[string]interface{}
    DataDir   string
    DB        *sql.DB
    AgentMgr  *agents.AgentManager
}

type TaskResult struct {
    Success    bool
    StatusText string
    ResultType string  // text / file / none
    Content    string
    FilePath   string
    Variables  map[string]interface{}
    Duration   int64
}

type ConfigField struct {
    Key         string `json:"key"`
    Label       string `json:"label"`
    Type        string `json:"type"` // text/textarea/select/number/boolean/date/time/multi_select
    Required    bool   `json:"required"`
    Default     string `json:"default,omitempty"`
    Options     []Option `json:"options,omitempty"`
    Placeholder string `json:"placeholder,omitempty"`
    Condition   string `json:"condition,omitempty"` // 显示条件，如 "output_type=file"
}

type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
```

### 6.3 调度配置转换

| 用户选择 | schedule_config | 转换Cron |
|---------|----------------|---------|
| 仅一次 2026-08-01 14:00 | `{"type":"once","datetime":"2026-08-01T14:00"}` | 使用time.AfterFunc |
| 每天 8:00 | `{"type":"daily","hour":8,"min":0}` | `0 8 * * *` |
| 每周一三五 9:00 | `{"type":"weekly","days":[1,3,5],"hour":9,"min":0}` | `0 9 * * 1,3,5` |
| 每月1号 10:00 | `{"type":"monthly","day":1,"hour":10,"min":0}` | `0 10 1 * *` |
| 自定义 | `{"type":"custom","cron":"0 8 * * 1-5"}` | 直接使用cron |

### 6.4 执行流程

```
1. Cron触发 / 依赖触发 / 手动触发
2. 创建 automation_executions 记录（status=running）
3. 获取 TaskExecutor
4. 替换config中的系统变量（{{date}}等）
5. 执行 executor.Execute()
6. 失败时检查重试:
   - retry_count < max_retries → 等待retry_interval后重新执行
   - 超过重试次数 → 标记failed
7. 更新 execution 记录
8. 检查 automation_dependencies，触发依赖任务
9. 发送应用内通知（如果任务有通知需求）
```

### 6.5 WorkflowExecutor 内部流程

```
1. 从 automation_steps 加载步骤（按step_index排序）
2. 初始化变量池
3. 顺序执行:
   a. 创建 step_execution 记录（status=running）
   b. 替换步骤config中的变量引用
   c. 执行步骤
   d. 将输出写入变量池
   e. 如果是condition步骤，根据结果决定下一步
   f. 更新 step_execution 记录
4. 返回最终结果
```

---

## 七、前端设计

### 7.1 页面结构

合并"任务"和"工具流"为统一的"自动化"页面，App.jsx tabs中移除 tasks 和 toolflow，新增 automation。

### 7.2 主页面

- 顶部: 标题 + 新建按钮
- 类型筛选标签栏: 全部/对话/搜索/报告/备份/提醒/监控/打卡/复习/清理/流程
- 统计栏: 运行中数/已暂停数/今日执行数
- 任务卡片列表: 名称+类型标签+调度信息+上次/下次执行时间+操作按钮
- workflow类型卡片额外显示"编辑流程"按钮

### 7.3 新建任务流程

1. 选择任务类型（10种卡片式选择）
2. 填写配置表单（根据类型动态渲染）
3. 设置调度（时间选择器替代Cron）
4. 创建（workflow类型自动跳转流程编辑器）

### 7.4 任务详情页

三个Tab:
- **配置**: 与创建时相同的表单，可编辑
- **执行记录**: 列表展示，含状态/耗时/结果预览/文件路径/错误信息
- **依赖关系**: 显示上游和下游依赖，可添加/删除

### 7.5 流程编辑器（workflow专用）

- 左侧: 步骤列表，可拖拽排序，每个步骤显示类型图标+名称
- 右侧: 选中步骤的配置面板，根据step_type渲染表单
- 步骤间箭头连线显示执行顺序
- "插入变量"按钮: 弹出前序步骤变量列表，点击插入

### 7.6 执行详情页

- 基本信息: 任务名/状态/总耗时/执行时间
- 步骤级进度: 每步的输入/输出/耗时/错误
- 结果操作: 打开文件/打开目录/重试/查看详细内容

---

## 八、API接口变更

### 8.1 删除的接口

| 原接口 | 说明 |
|-------|------|
| GetSchedules | 合并 |
| GetSchedule | 合并 |
| CreateSchedule | 合并 |
| UpdateSchedule | 合并 |
| DeleteSchedule | 合并 |
| ToggleSchedule | 合并 |
| RunScheduleNow | 合并 |
| GetScheduleExecutions | 合并 |
| GetToolFlows | 合并 |
| GetToolFlow | 合并 |
| CreateToolFlow | 合并 |
| UpdateToolFlow | 合并 |
| DeleteToolFlow | 合并 |
| ToggleToolFlow | 合并 |
| ExecuteToolFlow | 合并 |
| GetToolFlowExecutions | 合并 |

### 8.2 新增的接口

| 接口 | 参数 | 说明 |
|------|------|------|
| GetAutomationTasks | taskType string | 获取任务列表，taskType为空返回全部 |
| GetAutomationTask | id int | 获取单个任务 |
| CreateAutomationTask | name,desc,taskType,configJSON,scheduleType,scheduleConfigJSON,enabled | 创建任务 |
| UpdateAutomationTask | id,name,desc,taskType,configJSON,scheduleType,scheduleConfigJSON,enabled | 更新任务 |
| DeleteAutomationTask | id int | 删除任务 |
| ToggleAutomationTask | id int, enabled bool | 启用/禁用 |
| RunAutomationTaskNow | id int | 立即执行 |
| GetAutomationExecutions | taskID int | 获取执行记录 |
| GetAutomationSteps | taskID int | 获取workflow步骤 |
| SaveAutomationSteps | taskID int, stepsJSON string | 保存workflow步骤 |
| GetStepExecutions | executionID int | 获取步骤级执行详情 |
| GetAutomationDependencies | taskID int | 获取依赖关系 |
| AddAutomationDependency | taskID, dependsOnID int, condition string | 添加依赖 |
| RemoveAutomationDependency | id int | 删除依赖 |
| GetTaskConfigSchema | taskType string | 获取配置表单Schema |

---

## 九、代码改动清单

### 9.1 删除文件

| 文件 | 原因 |
|------|------|
| internal/scheduler/scheduler.go | 替换为 automation/engine.go |
| internal/toolflow/toolflow.go | 逻辑分散到 automation/ 各执行器 |
| internal/services/schedule_service.go | 替换为 automation_service.go |
| internal/services/toolflow_service.go | 合并入 automation_service.go |
| frontend/src/pages/TasksPage.jsx | 替换为 AutomationPage.jsx |
| frontend/src/pages/ToolFlowPage.jsx | 合并入 AutomationPage.jsx |

### 9.2 新增文件

| 文件 | 说明 |
|------|------|
| internal/automation/engine.go | 统一调度引擎 |
| internal/automation/executor.go | TaskExecutor接口 + Registry |
| internal/automation/agent_chat.go | AgentChatExecutor |
| internal/automation/web_search.go | WebSearchExecutor |
| internal/automation/report.go | ReportExecutor |
| internal/automation/backup.go | BackupExecutor |
| internal/automation/reminder.go | ReminderExecutor |
| internal/automation/monitor.go | MonitorExecutor |
| internal/automation/habit_checkin.go | HabitCheckinExecutor |
| internal/automation/review.go | ReviewExecutor |
| internal/automation/cleanup.go | CleanupExecutor |
| internal/automation/workflow.go | WorkflowExecutor |
| internal/services/automation_service.go | 统一数据服务 |
| frontend/src/pages/AutomationPage.jsx | 统一前端页面 |

### 9.3 修改文件

| 文件 | 改动 |
|------|------|
| internal/db/db.go | 替换4张旧表为5张新表，添加迁移逻辑 |
| internal/models/models.go | 替换Schedule/ToolFlow等为新模型 |
| app.go | 替换调度器初始化、Wails接口、移除江璐文案 |
| main.go | 无需改动 |
| tray.go | 合并两个菜单项为一个"自动化" |
| frontend/src/App.jsx | 合并tabs |
| internal/ai/prompt.go | 移除江璐人设，改为along助手 |
| internal/core/companion_core.go | 移除情绪标签中的姐姐语气 |

---

## 十、数据迁移策略

启动时在db.go的init逻辑中执行：

1. 检查 `schedules` 表是否存在
2. 存在则逐行迁移到 `automation_tasks`：
   - `action_type="agent_call"` → `task_type="agent_chat"`
   - `action_type="tool_flow"` → `task_type="workflow"`，同时从toolflows表迁移steps
   - `action_type="message"` → `task_type="reminder"`
3. 迁移 `schedule_executions` → `automation_executions`
4. 迁移 `toolflows` → `automation_tasks` + `automation_steps`
5. 迁移 `toolflow_executions` → `automation_executions`
6. 重命名旧表为 `_legacy` 后缀保留备份

---

## 十一、品牌变更

移除所有"江璐姐姐"相关内容，改为多Agent助手along定位：

- PRD.md: 重写产品定义
- 情绪标签: "温柔"→"专业", "心疼"→"关注", "安抚"→"支持"
- 话题建议: 姐姐语气→助手语气
- 系统提示词: 人格设定→助手能力设定
- 前端文案: 统一为along助手风格
