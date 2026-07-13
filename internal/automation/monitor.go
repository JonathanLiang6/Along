package automation

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// MonitorExecutor 网页监控执行器
type MonitorExecutor struct {
	agentMgr *agents.AgentManager
}

func (e *MonitorExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	url, _ := config["url"].(string)
	selector, _ := config["selector"].(string) // 预留选择器字段，当前抓取整页
	_ = selector

	if url == "" {
		return &models.TaskResult{Success: false, StatusText: "监控URL不能为空", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	url = services.ReplaceVariables(url, ctx.Variables)

	// 获取WebAgent并抓取页面内容
	webAgent, ok := e.agentMgr.GetAgent("web")
	if !ok {
		return &models.TaskResult{Success: false, StatusText: "找不到WebAgent", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	// 类型断言获取WebAgent具体类型以调用FetchPageContent
	webAgentImpl, ok := webAgent.(*agents.WebAgent)
	if !ok {
		return &models.TaskResult{Success: false, StatusText: "WebAgent类型断言失败", Duration: time.Since(startTime).Milliseconds()}, nil
	}

	currentContent, err := webAgentImpl.FetchPageContent(url)
	if err != nil {
		return &models.TaskResult{Success: false, StatusText: "抓取页面失败: " + err.Error(), Duration: time.Since(startTime).Milliseconds()}, nil
	}

	// 读取上次内容进行对比
	cacheFile := filepath.Join(ctx.DataDir, "monitor_cache", fmt.Sprintf("monitor_%d.txt", ctx.TaskID))
	os.MkdirAll(filepath.Dir(cacheFile), 0755)

	lastContent, _ := os.ReadFile(cacheFile)

	// 保存当前内容
	os.WriteFile(cacheFile, []byte(currentContent), 0644)

	// 对比内容
	if len(lastContent) > 0 && string(lastContent) == currentContent {
		return &models.TaskResult{
			Success:    true,
			StatusText: "页面内容无变化",
			ResultType: "text",
			Content:    "监控页面内容未发生变化",
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 内容有变化或首次监控
	statusText := "首次监控，已记录内容"
	if len(lastContent) > 0 {
		statusText = "检测到页面内容变化"
	}

	result := &models.TaskResult{
		Success:    true,
		StatusText: statusText,
		ResultType: "notify",
		Content:    fmt.Sprintf("监控URL: %s\n\n%s", url, truncate(currentContent, 500)),
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"monitor_url": url, "monitor_content": currentContent},
	}

	return result, nil
}

func (e *MonitorExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "url", Label: "监控URL", Type: "text", Required: true, Placeholder: "https://example.com"},
		{Key: "selector", Label: "内容选择器", Type: "text", Placeholder: "预留字段，当前抓取整页内容"},
	}
}
