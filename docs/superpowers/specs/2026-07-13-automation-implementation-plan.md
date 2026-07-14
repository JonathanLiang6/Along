# 统一自动化系统实施计划

> 关联设计: docs/superpowers/specs/2026-07-13-automation-system-design.md
> 项目根: e:\Projects_of_Liang\SingleProject\Go_Liang\Along

## 目标
将割裂的Scheduler和ToolFlow合并为统一的"自动化任务"系统，提供10种填空式任务类型、可视化流程编排、步骤级执行进度，并移除江璐姐姐人设。

## 前置条件
- [ ] 项目可编译: `wails dev` 能正常启动
- [ ] 数据库已有旧表 schedules/toolflows（启动应用自动创建）
- [ ] 备份数据库: `%APPDATA%\AICompanion\companion.db`

## 阶段划分

### 阶段1: 数据库与模型层
1. 更新 db.go 创建5张新表 + 迁移旧表
2. 更新 models.go 定义新结构体
3. 提交检查点

### 阶段2: 服务层
4. 创建 automation_service.go (统一数据服务)
5. 删除 schedule_service.go 和 toolflow_service.go
6. 提交检查点

### 阶段3: 执行引擎
7. 创建 automation/executor.go (接口+Registry)
8. 创建 automation/engine.go (调度引擎)
9. 创建10个执行器文件
10. 删除 scheduler/scheduler.go 和 toolflow/toolflow.go
11. 提交检查点

### 阶段4: App层
12. 更新 app.go (初始化 + Wails接口)
13. 更新 tray.go (合并菜单项)
14. 提交检查点

### 阶段5: 前端
15. 创建 AutomationPage.jsx (统一页面)
16. 删除 TasksPage.jsx 和 ToolFlowPage.jsx
17. 更新 App.jsx (tabs)
18. 提交检查点

### 阶段6: 品牌清理
19. 重写 PRD.md
20. 清理 prompt.go / companion_core.go / app.go 中的江璐内容
21. 提交检查点

### 阶段7: 验证
22. `wails build` 编译验证
23. 启动应用验证功能

## 回滚
- 数据库: 恢复 companion.db 备份
- 代码: `git reset --hard <checkpoint-commit>`
