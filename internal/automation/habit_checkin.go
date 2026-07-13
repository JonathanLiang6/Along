package automation

import (
	"database/sql"
	"fmt"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// HabitCheckinExecutor 习惯打卡执行器
type HabitCheckinExecutor struct {
	agentMgr *agents.AgentManager
	db       *sql.DB
}

func (e *HabitCheckinExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	goalID := 0
	if gid, ok := config["goal_id"].(float64); ok {
		goalID = int(gid)
	}
	content, _ := config["content"].(string)
	if content == "" {
		content = "自动打卡"
	}
	content = services.ReplaceVariables(content, ctx.Variables)
	mood, _ := config["mood"].(string)

	// 记录打卡
	planService := services.NewPlanService(e.db)
	var companionResponse string

	// 调用EmotionAgent生成鼓励话语
	agent, ok := e.agentMgr.GetAgent("emotion")
	if ok {
		agentCtx := agents.AgentContext{
			Content: "我今天完成了：" + content + "，请给我一些鼓励。",
			History: []ai.Message{},
		}
		response, err := agent.Process(agentCtx)
		if err == nil {
			companionResponse = response.Content
		}
	}

	if companionResponse == "" {
		companionResponse = "坚持就是胜利，做得很好！"
	}

	// 保存打卡记录
	if goalID > 0 {
		_, err := planService.AddCheckIn(goalID, content, mood, companionResponse)
		if err != nil {
			return &models.TaskResult{
				Success:    false,
				StatusText: "打卡记录保存失败: " + err.Error(),
				Duration:   time.Since(startTime).Milliseconds(),
			}, nil
		}
	}

	result := &models.TaskResult{
		Success:    true,
		StatusText: "打卡完成: " + truncate(content, 50),
		ResultType: "notify",
		Content:    fmt.Sprintf("打卡内容: %s\n\nAlong说: %s", content, companionResponse),
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"checkin_content": content, "companion_response": companionResponse},
	}

	return result, nil
}

func (e *HabitCheckinExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "goal_id", Label: "关联计划ID", Type: "number", Placeholder: "0表示不关联具体计划"},
		{Key: "content", Label: "打卡内容", Type: "textarea", Required: true, Placeholder: "支持 {{date}} {{weekday}} 等变量"},
		{Key: "mood", Label: "心情", Type: "select", Options: []models.ConfigOption{
			{Value: "", Label: "不记录"},
			{Value: "开心", Label: "开心"},
			{Value: "平静", Label: "平静"},
			{Value: "疲惫", Label: "疲惫"},
			{Value: "焦虑", Label: "焦虑"},
		}},
	}
}
