package automation

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// WorkflowExecutor 流程编排执行器
type WorkflowExecutor struct {
	agentMgr *agents.AgentManager
	db       *sql.DB
	dataDir  string
}

func (e *WorkflowExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	// 从DB加载步骤
	service := services.NewAutomationService(e.db)
	steps, err := service.GetSteps(ctx.TaskID)
	if err != nil || len(steps) == 0 {
		return &models.TaskResult{Success: false, StatusText: "没有配置步骤", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	variables := make(map[string]interface{})
	// 合并ctx传入的变量
	for k, v := range ctx.Variables {
		variables[k] = v
	}

	stepExecIndex := 0
	for stepExecIndex < len(steps) {
		step := steps[stepExecIndex]

		// 创建步骤执行记录
		stepExecID, _ := service.CreateStepExecution(ctx.ExecID, step.StepIndex, step.Name)

		stepStartTime := time.Now()

		// 解析步骤config中的变量
		stepConfig := make(map[string]interface{})
		if step.Config != "" {
			json.Unmarshal([]byte(step.Config), &stepConfig)
		}

		// 替换所有字符串值中的变量
		for k, v := range stepConfig {
			if s, ok := v.(string); ok {
				stepConfig[k] = services.ReplaceVariables(s, variables)
			}
		}

		// 执行步骤
		var stepResult *models.TaskResult
		var stepErr error
		switch step.StepType {
		case "agent":
			stepResult, stepErr = e.executeAgentStep(stepConfig, variables)
		case "search":
			stepResult, stepErr = e.executeSearchStep(stepConfig, variables)
		case "condition":
			stepResult, stepErr = e.executeConditionStep(stepConfig, variables, step)
		case "save_file":
			stepResult, stepErr = e.executeSaveFileStep(stepConfig, variables)
		case "notify":
			stepResult, stepErr = e.executeNotifyStep(stepConfig, variables)
		default:
			stepErr = fmt.Errorf("未知步骤类型: %s", step.StepType)
		}

		stepDuration := time.Since(stepStartTime).Milliseconds()

		if stepErr != nil {
			service.UpdateStepExecution(stepExecID, "failed", fmt.Sprintf("%v", stepConfig), "", stepErr.Error(), stepDuration)
			// 检查失败跳转
			if step.NextOnFailure == -1 {
				return &models.TaskResult{Success: false, StatusText: "步骤" + step.Name + "失败: " + stepErr.Error(), Duration: time.Since(startTime).Milliseconds()}, nil
			}
			if step.NextOnFailure > 0 {
				stepExecIndex = step.NextOnFailure - 1
				continue
			}
		} else if stepResult != nil {
			inputPreview := fmt.Sprintf("%v", stepConfig)
			outputPreview := stepResult.Content
			if len(outputPreview) > 200 {
				outputPreview = outputPreview[:200] + "..."
			}
			service.UpdateStepExecution(stepExecID, "success", inputPreview, outputPreview, "", stepDuration)

			// 保存输出变量
			if step.OutputVar != "" {
				variables[step.OutputVar] = stepResult.Content
			}

			// 条件步骤的跳转处理
			if step.StepType == "condition" {
				if stepResult.Success {
					if step.NextOnSuccess > 0 {
						stepExecIndex = step.NextOnSuccess - 1
						continue
					}
				} else {
					if step.NextOnFailure > 0 {
						stepExecIndex = step.NextOnFailure - 1
						continue
					} else if step.NextOnFailure == -1 {
						return &models.TaskResult{Success: true, StatusText: "条件不满足，流程结束", Duration: time.Since(startTime).Milliseconds(), Variables: variables}, nil
					}
				}
			}
		}

		stepExecIndex++
	}

	return &models.TaskResult{
		Success:    true,
		StatusText: "流程执行完成",
		ResultType: "text",
		Content:    fmt.Sprintf("执行了 %d 个步骤", len(steps)),
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  variables,
	}, nil
}

// executeAgentStep 执行Agent调用步骤
func (e *WorkflowExecutor) executeAgentStep(config map[string]interface{}, variables map[string]interface{}) (*models.TaskResult, error) {
	startTime := time.Now()

	agentName, _ := config["agent_name"].(string)
	content, _ := config["content"].(string)
	if content == "" {
		if c, ok := config["prompt"].(string); ok {
			content = c
		}
	}

	agent, ok := e.agentMgr.GetAgent(agentName)
	if !ok {
		return nil, fmt.Errorf("找不到Agent: %s", agentName)
	}

	// 构建Extra
	extra := make(map[string]interface{})
	for k, v := range config {
		if k != "agent_name" && k != "content" && k != "prompt" {
			extra[k] = v
		}
	}

	agentCtx := agents.AgentContext{
		Content: content,
		History: []ai.Message{},
		Extra:   extra,
	}
	response, err := agent.Process(agentCtx)
	if err != nil {
		return nil, err
	}

	return &models.TaskResult{
		Success:    true,
		StatusText: "Agent步骤完成",
		ResultType: "text",
		Content:    response.Content,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// executeSearchStep 执行搜索步骤
func (e *WorkflowExecutor) executeSearchStep(config map[string]interface{}, variables map[string]interface{}) (*models.TaskResult, error) {
	startTime := time.Now()

	query, _ := config["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("缺少搜索查询参数")
	}

	webAgent, ok := e.agentMgr.GetAgent("web")
	if !ok {
		return nil, fmt.Errorf("找不到WebAgent")
	}

	agentCtx := agents.AgentContext{
		Content: query,
		History: []ai.Message{},
	}
	response, err := webAgent.Process(agentCtx)
	if err != nil {
		return nil, err
	}

	return &models.TaskResult{
		Success:    true,
		StatusText: "搜索步骤完成",
		ResultType: "text",
		Content:    response.Content,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// executeConditionStep 执行条件判断步骤
func (e *WorkflowExecutor) executeConditionStep(config map[string]interface{}, variables map[string]interface{}, step models.AutomationStep) (*models.TaskResult, error) {
	startTime := time.Now()

	condition, _ := config["condition"].(string)
	if condition == "" {
		return &models.TaskResult{
			Success:    true,
			StatusText: "条件为空，默认通过",
			ResultType: "text",
			Content:    "true",
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 替换变量
	condition = services.ReplaceVariables(condition, variables)

	// 简单条件评估
	result := evaluateCondition(condition, variables)

	statusText := "条件满足"
	if !result {
		statusText = "条件不满足"
	}

	return &models.TaskResult{
		Success:    result,
		StatusText: statusText,
		ResultType: "text",
		Content:    fmt.Sprintf("%v", result),
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// executeSaveFileStep 执行保存文件步骤
func (e *WorkflowExecutor) executeSaveFileStep(config map[string]interface{}, variables map[string]interface{}) (*models.TaskResult, error) {
	startTime := time.Now()

	filePath, _ := config["file_path"].(string)
	content, _ := config["content"].(string)

	if filePath == "" {
		return nil, fmt.Errorf("缺少文件路径")
	}

	// 内容可以是直接配置的，也可以从变量中取
	if content == "" {
		if contentVar, ok := config["content_var"].(string); ok {
			if val, ok := variables[contentVar]; ok {
				content = fmt.Sprintf("%v", val)
			}
		}
	}

	dir := filepath.Dir(filePath)
	os.MkdirAll(dir, 0755)

	err := os.WriteFile(filePath, []byte(content), 0644)
	if err != nil {
		return nil, fmt.Errorf("保存文件失败: %w", err)
	}

	return &models.TaskResult{
		Success:    true,
		StatusText: "文件已保存: " + filePath,
		ResultType: "file",
		FilePath:   filePath,
		Content:    content,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// executeNotifyStep 执行通知步骤
func (e *WorkflowExecutor) executeNotifyStep(config map[string]interface{}, variables map[string]interface{}) (*models.TaskResult, error) {
	startTime := time.Now()

	message, _ := config["message"].(string)
	if message == "" {
		message = "流程通知"
	}

	// 支持从变量取内容
	if msgVar, ok := config["message_var"].(string); ok {
		if val, ok := variables[msgVar]; ok {
			message = fmt.Sprintf("%v", val)
		}
	}

	return &models.TaskResult{
		Success:    true,
		StatusText: "通知: " + truncate(message, 100),
		ResultType: "notify",
		Content:    message,
		Duration:   time.Since(startTime).Milliseconds(),
	}, nil
}

// evaluateCondition 简单条件评估
func evaluateCondition(condition string, variables map[string]interface{}) bool {
	condition = strings.TrimSpace(condition)

	// 直接布尔值
	if condition == "true" || condition == "1" {
		return true
	}
	if condition == "false" || condition == "0" || condition == "" {
		return false
	}

	// 检查相等条件: var == "value"
	if parts := strings.SplitN(condition, "==", 2); len(parts) == 2 {
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		right = strings.Trim(right, "\"'")
		return left == right
	}

	// 检查不等条件: var != "value"
	if parts := strings.SplitN(condition, "!=", 2); len(parts) == 2 {
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		right = strings.Trim(right, "\"'")
		return left != right
	}

	// 检查包含条件: var contains "value"
	if parts := strings.SplitN(condition, "contains", 2); len(parts) == 2 {
		left := strings.TrimSpace(parts[0])
		right := strings.TrimSpace(parts[1])
		right = strings.Trim(right, "\"'")
		return strings.Contains(left, right)
	}

	// 默认：非空即为true
	return condition != ""
}

func (e *WorkflowExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "_notice", Label: "提示", Type: "text", Placeholder: "流程步骤请在步骤管理页面配置"},
	}
}
