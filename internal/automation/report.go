package automation

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// ReportExecutor 报告生成执行器
type ReportExecutor struct {
	agentMgr *agents.AgentManager
	db       *sql.DB
	dataDir  string
}

func (e *ReportExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	reportType, _ := config["report_type"].(string)
	if reportType == "" {
		reportType = "weekly"
	}
	outputType, _ := config["output_type"].(string)
	if outputType == "" {
		outputType = "file"
	}
	filePath, _ := config["file_path"].(string)
	if filePath == "" {
		filePath = filepath.Join(e.dataDir, "reports", fmt.Sprintf("%s_%s.md", reportType, time.Now().Format("2006-01-02")))
	}

	// 根据reportType计算时间范围
	now := time.Now()
	var startDate, endDate string

	switch reportType {
	case "daily":
		startDate = now.AddDate(0, 0, -1).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	case "monthly":
		startDate = now.AddDate(0, -1, 0).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	default:
		startDate = now.AddDate(0, 0, -7).Format("2006-01-02")
		endDate = now.Format("2006-01-02")
	}

	// 从DB读取近期对话
	convService := services.NewConversationService(e.db)
	recentMsgs, _ := convService.GetRecentMessages(50)

	// 从DB读取记忆
	memService := services.NewMemoryService(e.db)
	memories, _ := memService.GetMemories("")

	// 构建上下文
	var chatCtx string
	for _, msg := range recentMsgs {
		role := "用户"
		if msg.Role == "assistant" {
			role = "Along"
		}
		chatCtx += fmt.Sprintf("%s: %s\n", role, msg.Content)
	}

	var memCtx string
	for _, m := range memories {
		memCtx += fmt.Sprintf("- [%s] %s\n", m.Type, m.Content)
	}

	// 调用ReflectionAgent生成报告
	agent, ok := e.agentMgr.GetAgent("reflection")
	if !ok {
		return &models.TaskResult{Success: false, StatusText: "找不到ReflectionAgent", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	prompt := fmt.Sprintf("请生成%s复盘报告（周期：%s 至 %s）。\n\n近期对话：\n%s\n\n记忆：\n%s",
		reportType, startDate, endDate, chatCtx, memCtx)
	prompt = services.ReplaceVariables(prompt, ctx.Variables)

	agentCtx := agents.AgentContext{
		Content: prompt,
		History: []ai.Message{},
	}
	response, err := agent.Process(agentCtx)
	if err != nil {
		return &models.TaskResult{Success: false, StatusText: "报告生成失败: " + err.Error(), Duration: time.Since(startTime).Milliseconds()}, nil
	}

	content := response.Content
	result := &models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("%s报告生成完成", reportType),
		ResultType: "text",
		Content:    content,
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"report": content, "report_type": reportType},
	}

	// 处理输出
	if outputType == "file" {
		filePath = services.ReplaceVariables(filePath, ctx.Variables)
		dir := filepath.Dir(filePath)
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filePath, []byte(content), 0644); err == nil {
			result.FilePath = filePath
			result.StatusText = "报告已保存到 " + filePath
		}
	} else if outputType == "notify" {
		result.StatusText = "报告已生成: " + truncate(content, 100)
	}

	return result, nil
}

func (e *ReportExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "report_type", Label: "报告类型", Type: "select", Required: true, Default: "weekly", Options: []models.ConfigOption{
			{Value: "daily", Label: "日报"},
			{Value: "weekly", Label: "周报"},
			{Value: "monthly", Label: "月报"},
		}},
		{Key: "output_type", Label: "输出方式", Type: "select", Default: "file", Options: []models.ConfigOption{
			{Value: "record", Label: "仅记录"},
			{Value: "notify", Label: "通知我"},
			{Value: "file", Label: "保存到文件"},
		}},
		{Key: "file_path", Label: "文件保存路径", Type: "text", Condition: "output_type=file", Placeholder: "留空则自动生成"},
	}
}
