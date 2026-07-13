package automation

import (
	"time"

	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// ReminderExecutor 提醒执行器
type ReminderExecutor struct {
}

func (e *ReminderExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	content, _ := config["content"].(string)
	if content == "" {
		content = "这是一个提醒"
	}
	content = services.ReplaceVariables(content, ctx.Variables)

	result := &models.TaskResult{
		Success:    true,
		StatusText: "提醒: " + truncate(content, 100),
		ResultType: "notify",
		Content:    content,
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"reminder": content},
	}

	return result, nil
}

func (e *ReminderExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "content", Label: "提醒内容", Type: "textarea", Required: true, Placeholder: "支持 {{date}} {{time}} {{weekday}} 等变量"},
	}
}
