package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"strings"
)

type TechAnalysisAgent struct {
	BaseAgent
	webAgent *WebAgent
}

func NewTechAnalysisAgent(aiClient *ai.Client, webAgent *WebAgent) *TechAnalysisAgent {
	return &TechAnalysisAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "tech_analysis",
			desc:     "技术分析：深入分析AI技术概念，不明白时联网查找",
		},
		webAgent: webAgent,
	}
}

func (ta *TechAnalysisAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"什么是", "解释", "分析", "原理", "机制", "架构", "技术", "算法",
		"模型", "框架", "系统", "原理", "如何工作", "工作原理",
		"Loop", "Agentic", "RAG", "大模型", "微调", "推理",
		"Transformer", "LLM", "多模态", "扩散模型", "强化学习",
	}
	return KeywordMatch(ctx.Content, keywords)
}

func (ta *TechAnalysisAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if ta.aiClient == nil {
		return &AgentResult{
			Content: "我可以帮你分析AI技术概念。你想了解什么技术？",
			Emotion: "专业",
		}, nil
	}

	content := ctx.Content

	searchResults, searchErr := ta.performSearchIfNeeded(content)

	var contextInfo string
	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = ta.formatSearchResults(searchResults)
	}

	systemPrompt := `你是一个专业的AI技术分析专家。请对用户提出的技术概念进行深入分析，包括：
1. 技术定义与核心概念
2. 工作原理与机制
3. 应用场景
4. 优缺点分析
5. 最新发展与趋势

如果提供了搜索结果，请基于搜索结果进行分析，并引用来源。`

	userMessage := content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索到的相关信息：\n%s\n\n请分析：%s", contextInfo, content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	resp, err := ta.aiClient.Chat(messages, ai.WithTemperature(0.6))
	if err != nil {
		return &AgentResult{
			Content: fmt.Sprintf("分析遇到了一点问题，不过我可以尝试直接回答：%s", ta.directAnswer(content)),
			Emotion: "专业",
		}, nil
	}

	return &AgentResult{
		Content:      resp,
		Emotion:      "专业",
		ShouldRecord: true,
	}, nil
}

func (ta *TechAnalysisAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if ta.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我可以帮你分析AI技术概念。你想了解什么技术？", Done: true})
		}
		return nil
	}

	content := ctx.Content

	searchResults, searchErr := ta.performSearchIfNeeded(content)

	var contextInfo string
	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = ta.formatSearchResults(searchResults)
	}

	systemPrompt := `你是一个专业的AI技术分析专家。请对用户提出的技术概念进行深入分析，包括：
1. 技术定义与核心概念
2. 工作原理与机制
3. 应用场景
4. 优缺点分析
5. 最新发展与趋势

如果提供了搜索结果，请基于搜索结果进行分析，并引用来源。`

	userMessage := content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索到的相关信息：\n%s\n\n请分析：%s", contextInfo, content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	return ta.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.6))
}

func (ta *TechAnalysisAgent) performSearchIfNeeded(content string) ([]SearchResult, error) {
	if ta.webAgent == nil {
		return nil, fmt.Errorf("web agent not available")
	}

	searchKeywords := []string{
		"Loop", "Agentic", "RAG", "大模型", "LLM", "Transformer",
		"多模态", "扩散模型", "强化学习", "微调", "推理", "架构",
	}

	needsSearch := false
	for _, kw := range searchKeywords {
		if strings.Contains(strings.ToLower(content), strings.ToLower(kw)) {
			needsSearch = true
			break
		}
	}

	if !needsSearch {
		if strings.Contains(content, "最新") || strings.Contains(content, "最近") ||
			strings.Contains(content, "2026") || strings.Contains(content, "进展") {
			needsSearch = true
		}
	}

	if !needsSearch {
		return nil, nil
	}

	return ta.webAgent.search(content)
}

func (ta *TechAnalysisAgent) formatSearchResults(results []SearchResult) string {
	var sb strings.Builder
	sb.WriteString("搜索到的技术资料：\n\n")

	for i, result := range results {
		if i >= 5 {
			break
		}
		if result.Title != "" {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		}
		if result.Snippet != "" {
			snippet := result.Snippet
			if len(snippet) > 300 {
				snippet = snippet[:300] + "..."
			}
			sb.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
		if result.Link != "" {
			sb.WriteString(fmt.Sprintf("   来源: %s\n", result.Link))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func (ta *TechAnalysisAgent) directAnswer(content string) string {
	return fmt.Sprintf("关于「%s」的技术分析：\n\n这是一个有趣的技术话题。该技术涉及多个方面，包括核心原理、应用场景和发展趋势。如果你能提供更多具体问题，我可以给出更详细的分析。", content)
}
