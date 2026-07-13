package automation

import (
	"os"
	"path/filepath"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// AgentChatExecutor Agent对话执行器
type AgentChatExecutor struct {
	agentMgr *agents.AgentManager
}

func (e *AgentChatExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	agentName, _ := config["agent_name"].(string)
	prompt, _ := config["prompt"].(string)
	outputType, _ := config["output_type"].(string)
	filePath, _ := config["file_path"].(string)

	// 替换变量
	prompt = services.ReplaceVariables(prompt, ctx.Variables)

	// 调用Agent
	agent, ok := e.agentMgr.GetAgent(agentName)
	if !ok {
		return &models.TaskResult{
			Success:    false,
			StatusText: "找不到Agent: " + agentName,
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	agentCtx := agents.AgentContext{
		Content: prompt,
		History: []ai.Message{},
	}
	response, err := agent.Process(agentCtx)
	if err != nil {
		return &models.TaskResult{
			Success:    false,
			StatusText: "Agent调用失败: " + err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	content := response.Content
	result := &models.TaskResult{
		Success:    true,
		StatusText: "Agent对话完成",
		ResultType: "text",
		Content:    content,
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"result": content},
	}

	// 处理输出
	switch outputType {
	case "file":
		filePath = services.ReplaceVariables(filePath, ctx.Variables)
		dir := filepath.Dir(filePath)
		os.MkdirAll(dir, 0755)
		err := os.WriteFile(filePath, []byte(content), 0644)
		if err != nil {
			result.StatusText = "内容已生成但保存文件失败: " + err.Error()
		} else {
			result.FilePath = filePath
			result.StatusText = "已保存到 " + filePath
		}
	case "notify":
		result.StatusText = "Agent回复: " + truncate(content, 100)
	}

	return result, nil
}

func (e *AgentChatExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "agent_name", Label: "选择Agent", Type: "select", Required: true, Options: []models.ConfigOption{
			{Value: "web", Label: "Web搜索Agent"},
			{Value: "planner", Label: "规划Agent"},
			{Value: "emotion", Label: "情绪Agent"},
			{Value: "memory", Label: "记忆Agent"},
			{Value: "reflection", Label: "反思Agent"},
			{Value: "summarize", Label: "总结Agent"},
			{Value: "tool", Label: "工具Agent"},
		}},
		{Key: "prompt", Label: "提示词", Type: "textarea", Required: true, Placeholder: "输入给Agent的内容，支持 {{date}} {{time}} 变量"},
		{Key: "output_type", Label: "输出方式", Type: "select", Required: true, Default: "record", Options: []models.ConfigOption{
			{Value: "record", Label: "仅记录"},
			{Value: "notify", Label: "通知我"},
			{Value: "file", Label: "保存到文件"},
		}},
		{Key: "file_path", Label: "文件保存路径", Type: "text", Condition: "output_type=file", Placeholder: "D:\\Reports\\{{date}}.md"},
	}
}
