package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// WebSearchExecutor 信息检索执行器
type WebSearchExecutor struct {
	agentMgr *agents.AgentManager
}

func (e *WebSearchExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	query, _ := config["query"].(string)
	engine, _ := config["engine"].(string)
	if engine == "" {
		engine = "duckduckgo"
	}
	resultCount := 5
	if rc, ok := config["result_count"].(float64); ok {
		resultCount = int(rc)
	}
	needSummary, _ := config["need_summary"].(bool)
	outputType, _ := config["output_type"].(string)
	filePath, _ := config["file_path"].(string)

	query = services.ReplaceVariables(query, ctx.Variables)

	// 调用WebAgent搜索
	webAgent, ok := e.agentMgr.GetAgent("web")
	if !ok {
		return &models.TaskResult{Success: false, StatusText: "找不到WebAgent", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	agentCtx := agents.AgentContext{
		Content: query,
		History: []ai.Message{},
	}
	response, err := webAgent.Process(agentCtx)
	if err != nil {
		return &models.TaskResult{Success: false, StatusText: "搜索失败: " + err.Error(), Duration: time.Since(startTime).Milliseconds()}, nil
	}

	searchResult := response.Content

	// AI总结
	if needSummary {
		summarizeAgent, ok := e.agentMgr.GetAgent("summarize")
		if ok {
			sumCtx := agents.AgentContext{
				Content: "请总结以下搜索结果：\n" + searchResult,
				History: []ai.Message{},
				Extra:   map[string]interface{}{"raw_content": searchResult},
			}
			resp, err := summarizeAgent.Process(sumCtx)
			if err == nil {
				searchResult = resp.Content
			}
		}
	}

	result := &models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("搜索完成，返回%d条结果", resultCount),
		ResultType: "text",
		Content:    searchResult,
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"search_result": searchResult},
	}

	// 处理输出
	switch outputType {
	case "file":
		filePath = services.ReplaceVariables(filePath, ctx.Variables)
		dir := filepath.Dir(filePath)
		os.MkdirAll(dir, 0755)
		if err := os.WriteFile(filePath, []byte(searchResult), 0644); err == nil {
			result.FilePath = filePath
			result.StatusText = "已保存到 " + filePath
		}
	case "notify":
		result.StatusText = "搜索完成: " + truncate(searchResult, 100)
	}

	return result, nil
}

func (e *WebSearchExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "query", Label: "搜索关键词", Type: "text", Required: true, Placeholder: "支持 {{date}} {{weekday}} 等变量"},
		{Key: "engine", Label: "搜索引擎", Type: "select", Default: "duckduckgo", Options: []models.ConfigOption{
			{Value: "duckduckgo", Label: "DuckDuckGo"},
			{Value: "bing", Label: "Bing"},
		}},
		{Key: "result_count", Label: "结果数量", Type: "number", Default: "5"},
		{Key: "need_summary", Label: "AI总结", Type: "boolean", Default: "true"},
		{Key: "output_type", Label: "输出方式", Type: "select", Required: true, Default: "record", Options: []models.ConfigOption{
			{Value: "record", Label: "仅记录"},
			{Value: "notify", Label: "通知我"},
			{Value: "file", Label: "保存到文件"},
		}},
		{Key: "file_path", Label: "文件保存路径", Type: "text", Condition: "output_type=file", Placeholder: "D:\\Reports\\tech_{{date}}.md"},
	}
}
