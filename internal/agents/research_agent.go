package agents

import (
	"ai-companion/internal/ai"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ResearchAgent 搜索与调研 Agent
type ResearchAgent struct {
	BaseAgent
	httpClient *http.Client
}

// NewResearchAgent 创建调研 Agent
func NewResearchAgent(aiClient *ai.Client) *ResearchAgent {
	return &ResearchAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "research",
			desc:     "搜索调研：网络搜索、信息查询、知识问答",
		},
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Match 计算匹配度
func (ra *ResearchAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"搜索", "查一下", "了解一下", "调研", "研究", "什么是", "怎么",
		"为什么", "如何", "最新", "新闻", "信息", "资料", "百度", "谷歌",
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

	searchResults, searchErr := ra.webSearch(ctx.Content)
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

	searchResults, searchErr := ra.webSearch(ctx.Content)
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

// webSearch 执行网络搜索（使用 DuckDuckGo Instant Answer API）
func (ra *ResearchAgent) webSearch(query string) ([]SearchResult, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))

	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := ra.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("搜索请求失败: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ddgResp DuckDuckGoResponse
	if err := json.Unmarshal(body, &ddgResp); err != nil {
		return nil, err
	}

	var results []SearchResult

	if ddgResp.AbstractText != "" {
		results = append(results, SearchResult{
			Title:   ddgResp.Heading,
			Link:    ddgResp.AbstractURL,
			Snippet: ddgResp.AbstractText,
		})
	}

	for _, topic := range ddgResp.RelatedTopics {
		if topic.Text != "" {
			results = append(results, SearchResult{
				Title:   extractTitle(topic.Text),
				Link:    topic.FirstURL,
				Snippet: topic.Text,
			})
		}
		if len(results) >= 5 {
			break
		}
	}

	return results, nil
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

// extractTitle 从文本中提取标题
func extractTitle(text string) string {
	parts := strings.SplitN(text, " - ", 2)
	if len(parts) > 0 {
		title := strings.TrimSpace(parts[0])
		if len(title) > 50 {
			return title[:50] + "..."
		}
		return title
	}
	if len(text) > 50 {
		return text[:50] + "..."
	}
	return text
}

// SearchResult 搜索结果
type SearchResult struct {
	Title   string `json:"title"`
	Link    string `json:"link"`
	Snippet string `json:"snippet"`
}

// DuckDuckGoResponse DuckDuckGo API 响应
type DuckDuckGoResponse struct {
	AbstractText   string `json:"AbstractText"`
	AbstractURL    string `json:"AbstractURL"`
	AbstractSource string `json:"AbstractSource"`
	Heading        string `json:"Heading"`
	RelatedTopics  []struct {
		Text     string `json:"Text"`
		FirstURL string `json:"FirstURL"`
	} `json:"RelatedTopics"`
}
