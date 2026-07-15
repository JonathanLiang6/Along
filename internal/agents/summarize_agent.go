package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"strings"
	"time"
)

// SummarizeAgent 信息整合 Agent
// 职责：接收各种原始信息（搜索结果、对话内容、粘贴文本），进行去重、分类、提炼，输出结构化摘要
// 不负责生成文件，只负责内容整合
type SummarizeAgent struct {
	BaseAgent
	webAgent *WebAgent
}

// NewSummarizeAgent 创建信息整合 Agent
func NewSummarizeAgent(aiClient *ai.Client, webAgent *WebAgent) *SummarizeAgent {
	return &SummarizeAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "summarize",
			desc:     "信息整合：对搜索结果、对话内容、原始文本进行去重、分类、提炼，输出结构化摘要",
		},
		webAgent: webAgent,
	}
}

// Match 计算匹配度
func (sa *SummarizeAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"总结", "整理", "归纳", "梳理", "摘要", "提炼",
		"汇总", "概括", "整合", "分类整理",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (sa *SummarizeAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if sa.aiClient == nil {
		return &AgentResult{
			Content: "我可以帮你整合信息。请提供需要整理的内容或主题。",
			Emotion: "认真",
		}, nil
	}

	// 判断输入来源：如果 Extra 中有 raw_content，直接整理；否则尝试搜索
	rawContent := ""
	if rc, ok := ctx.Extra["raw_content"].(string); ok && rc != "" {
		rawContent = rc
	}

	var sourceText string
	var searchResults []SearchResult

	if rawContent != "" {
		// 直接整理用户提供的文本
		sourceText = rawContent
	} else {
		// 需要先搜索再整理
		query := ctx.Content
		if extraQuery, ok := ctx.Extra["query"].(string); ok && extraQuery != "" {
			query = extraQuery
		}

		results, err := sa.multiSearch(query)
		if err == nil && len(results) > 0 {
			searchResults = results
			sourceText = sa.formatSearchResults(results)
		} else {
			sourceText = "（未能获取到搜索结果，请基于已有知识整理）"
		}
	}

	// 整理类型：brief（简报）/ detailed（详报）/ technical（技术报告）
	summaryType := "detailed"
	if st, ok := ctx.Extra["summary_type"].(string); ok && st != "" {
		summaryType = st
	}

	summary, err := sa.generateSummary(ctx.Content, sourceText, summaryType)
	if err != nil {
		return &AgentResult{
			Content: fmt.Sprintf("整合信息时遇到问题：%v", err),
			Emotion: "抱歉",
		}, nil
	}

	return &AgentResult{
		Content:      summary,
		Emotion:      "认真",
		ShouldRecord: true,
		Data: map[string]interface{}{
			"summary_type":   summaryType,
			"search_results": searchResults,
			"source_length":  len(sourceText),
		},
	}, nil
}

// ProcessStream 流式处理
func (sa *SummarizeAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if sa.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我可以帮你整合信息。请提供需要整理的内容或主题。", Done: true})
		}
		return nil
	}

	result, err := sa.Process(ctx)
	if err != nil {
		return err
	}

	if callback != nil {
		callback(ai.StreamChunk{Content: result.Content, Done: true})
	}
	return nil
}

// multiSearch 多角度搜索并去重
func (sa *SummarizeAgent) multiSearch(query string) ([]SearchResult, error) {
	var allResults []SearchResult

	queries := []string{
		query,
		query + " 最新进展 2026",
		query + " 技术趋势",
	}

	for _, q := range queries {
		results, err := sa.webAgent.search(q)
		if err == nil && len(results) > 0 {
			allResults = append(allResults, results...)
		}
		time.Sleep(200 * time.Millisecond)
	}

	// 去重
	seen := make(map[string]bool)
	unique := []SearchResult{}
	for _, r := range allResults {
		if !seen[r.Link] && r.Snippet != "" {
			seen[r.Link] = true
			unique = append(unique, r)
			if len(unique) >= 12 {
				break
			}
		}
	}

	return unique, nil
}

// formatSearchResults 将搜索结果格式化为文本
func (sa *SummarizeAgent) formatSearchResults(results []SearchResult) string {
	var sb strings.Builder
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n   来源: %s\n\n", i+1, r.Title, r.Snippet, r.Link))
	}
	return sb.String()
}

// generateSummary 调用 AI 生成结构化摘要
func (sa *SummarizeAgent) generateSummary(topic, sourceText, summaryType string) (string, error) {
	if sa.aiClient == nil {
		return "", fmt.Errorf("AI客户端未初始化")
	}

	systemPrompt := ai.BuildSystemPrompt("summarize", "")

	var structureGuide string
	switch summaryType {
	case "brief":
		structureGuide = `请按以下结构输出简报：
1. **一句话概括**：用一句话总结核心信息
2. **关键要点**：3-5个要点，每个不超过两句话
3. **值得关注的信号**：1-2条值得持续关注的方向`
	case "technical":
		structureGuide = `请按以下结构输出技术报告摘要：
1. **概述**：2-3句话概括
2. **技术方向**：分领域列出主要技术进展
3. **代表项目/论文**：列出重要项目或论文及简要说明
4. **技术成熟度评估**：对关键技术的成熟度做简要评价
5. **来源清单**：列出信息来源`
	default: // detailed
		structureGuide = `请按以下结构输出详细摘要：
1. **核心摘要**：3-5句话概括最重要信息
2. **主要发现**：分点列出关键发现，每点包含具体细节
3. **分类整理**：按主题或领域分类归纳
4. **值得关注的趋势**：指出值得关注的发展方向
5. **信息来源**：列出参考来源`
	}

	userMessage := fmt.Sprintf(`主题：%s

原始信息：
%s

%s

请基于以上信息进行整合梳理。要求：
- 去重：相同信息只保留一次
- 分类：按主题归类
- 提炼：提取核心观点，去掉冗余描述
- 客观：基于原始信息整理，不添加臆测
- 语言：中文`, topic, sourceText, structureGuide)

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	resp, err := sa.aiClient.Chat(messages, ai.WithTemperature(0.5), ai.WithMaxTokens(2500))
	if err != nil {
		return "", err
	}

	return resp, nil
}
