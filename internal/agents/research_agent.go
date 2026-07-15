package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"strings"
	"time"
)

// ResearchAgent 深度调研 Agent
// 职责：多轮搜索 + 交叉验证 + 结构化输出的深度调研
type ResearchAgent struct {
	BaseAgent
	webAgent *WebAgent
}

// NewResearchAgent 创建调研 Agent
func NewResearchAgent(aiClient *ai.Client, webAgent *WebAgent) *ResearchAgent {
	return &ResearchAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "research",
			desc:     "深度调研：多轮搜索、交叉验证、结构化输出",
		},
		webAgent: webAgent,
	}
}

func (ra *ResearchAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "research", Description: "深度调研：多角度搜索、交叉验证", InputDesc: "调研主题", OutputDesc: "结构化调研结果"},
	}
}

// Match 计算匹配度
func (ra *ResearchAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"深度调研", "专题研究", "文献综述", "全面了解", "深入分析",
		"系统性研究", "综合调研", "调查报告",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (ra *ResearchAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if ra.aiClient == nil {
		return &AgentResult{
			Content: "调研是个耐心活。我们可以一起梳理思路，把问题拆清楚。你想了解什么？",
			Emotion: "认真",
		}, nil
	}

	searchResults, searchErr := ra.deepResearch(ctx.Content)
	var contextInfo string

	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = ra.summarizeSearchResults(searchResults)
	} else {
		contextInfo = "（网络搜索暂时不可用，我将基于我的知识来回答）"
	}

	systemPrompt := ai.BuildSystemPrompt("research", "")
	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索上下文：\n%s\n\n用户问题：%s", contextInfo, ctx.Content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	resp, err := ra.aiClient.Chat(messages, ai.WithTemperature(0.7))
	if err != nil {
		return &AgentResult{
			Content: "调研是个耐心活。我们可以一起梳理思路，把问题拆清楚。你想了解什么？",
			Emotion: "认真",
		}, nil
	}

	return &AgentResult{
		Content:      resp,
		Emotion:      "认真",
		ShouldRecord: true,
		Data:         searchResults,
	}, nil
}

// ProcessStream 流式处理
func (ra *ResearchAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if ra.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "调研是个耐心活。我们可以一起梳理思路，把问题拆清楚。你想了解什么？", Done: true})
		}
		return nil
	}

	searchResults, searchErr := ra.deepResearch(ctx.Content)
	var contextInfo string

	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = ra.summarizeSearchResults(searchResults)
	} else {
		contextInfo = "（网络搜索暂时不可用，我将基于我的知识来回答）"
	}

	systemPrompt := ai.BuildSystemPrompt("research", "")
	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索上下文：\n%s\n\n用户问题：%s", contextInfo, ctx.Content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	return ra.aiClient.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.7))
}

// Handle 处理调研（兼容旧接口）
func (ra *ResearchAgent) Handle(content string, context []ai.Message) (string, error) {
	ctx := AgentContext{
		Content: content,
		History: context,
	}
	result, err := ra.Process(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// deepResearch 深度调研：多角度搜索并去重
func (ra *ResearchAgent) deepResearch(query string) ([]SearchResult, error) {
	var allResults []SearchResult

	queries := []string{
		query,
		query + " research paper",
		query + " technical analysis",
		query + " latest developments",
		query + " 2026",
	}

	for i, q := range queries {
		if ra.webAgent != nil {
			results, err := ra.webAgent.search(q)
			if err == nil && len(results) > 0 {
				allResults = append(allResults, results...)
			}
		}
		if i < len(queries)-1 {
			time.Sleep(300 * time.Millisecond)
		}
	}

	seen := make(map[string]bool)
	unique := []SearchResult{}
	for _, r := range allResults {
		if !seen[r.Link] && r.Snippet != "" {
			seen[r.Link] = true
			unique = append(unique, r)
			if len(unique) >= 15 {
				break
			}
		}
	}

	return unique, nil
}

// summarizeSearchResults 将搜索结果摘要为上下文信息
func (ra *ResearchAgent) summarizeSearchResults(results []SearchResult) string {
	var sb strings.Builder
	sb.WriteString("以下是搜索到的相关信息：\n\n")

	for i, result := range results {
		if result.Title != "" {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		}
		if result.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", result.Snippet))
		}
		if result.Link != "" {
			sb.WriteString(fmt.Sprintf("   来源: %s\n", result.Link))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// SearchResult 搜索结果（复用web_agent的类型）
type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}
