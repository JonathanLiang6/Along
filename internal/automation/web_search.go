package automation

import (
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

	// AI总结 - 生成学术化结构化报告
	var reportContent string
	if needSummary {
		summarizeAgent, ok := e.agentMgr.GetAgent("summarize")
		if ok {
			sumCtx := agents.AgentContext{
				Content: "请将以下搜索结果整理成一份学术化的技术调研报告，包含以下部分：\n1. 技术概述与核心概念\n2. 研究现状与关键突破（引用重要论文和研究成果）\n3. 技术架构与实现原理\n4. 应用场景与商业化潜力\n\n请确保报告具有学术严谨性，引用关键文献和数据来源。\n\n搜索结果：\n" + searchResult,
				History: []ai.Message{},
				Extra:   map[string]interface{}{"raw_content": searchResult},
			}
			resp, err := summarizeAgent.Process(sumCtx)
			if err == nil {
				searchResult = resp.Content
			}
		}

		// 构建完整的调研文档
		now := time.Now()
		reportContent = fmt.Sprintf(`# AI前沿技术调研报告

**调研日期**: %s
**搜索关键词**: %s
**搜索引擎**: %s
**执行时间**: %s

---

## 摘要

本报告基于最新网络搜索结果，对当前AI领域的前沿技术进行系统性调研与分析，涵盖技术原理、研究进展、应用前景等方面。

---

## 一、技术概述与核心概念

%s

---

## 二、研究现状与关键突破

本部分汇总了近期重要的学术研究成果和技术突破：

---

## 三、技术架构与实现原理

深入分析核心技术的架构设计和实现机制。

---

## 四、应用场景与商业化潜力

探讨技术在各行业的应用可能性和市场前景。

---

## 参考文献

以下为报告引用的主要信息来源：

%s

---

*本报告由AI自动化调研系统生成*
`, now.Format("2006年1月2日 15:04"), query, engine, now.Format("2006-01-02 15:04:05"), searchResult, extractRawResults(searchResult))
	} else {
		reportContent = searchResult
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
		contentToWrite := reportContent
		if !needSummary {
			contentToWrite = fmt.Sprintf(`# AI前沿知识调研

**调研日期**: %s
**搜索关键词**: %s

---

%s
`, time.Now().Format("2006年1月2日"), query, searchResult)
		}
		if err := os.WriteFile(filePath, []byte(contentToWrite), 0644); err == nil {
			result.FilePath = filePath
			result.StatusText = "已保存到 " + filePath
		}
	case "notify":
		result.StatusText = "搜索完成: " + truncate(searchResult, 100)
	}

	return result, nil
}

func extractRawResults(content string) string {
	if strings.Contains(content, "搜索结果") || strings.Contains(content, "来源:") {
		return content
	}
	return "（原始搜索结果已整合到总结中）"
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
