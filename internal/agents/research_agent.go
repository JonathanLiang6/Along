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

// Process 深度调研处理：多角度搜索 → 正文抓取 → AI 综合分析
func (ra *ResearchAgent) Process(ctx AgentContext) (*AgentResult, error) {
	client := ra.GetAIClient()
	if client == nil {
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

	systemPrompt := `你是一位资深技术研究员。请基于提供的搜索材料，对用户的问题进行深度分析和总结。

输出要求：
1. **核心观点** - 提炼最重要的发现（3-5条）
2. **详细分析** - 按主题分类深入展开，引用搜索材料中的信息
3. **不同观点对比** - 如有不同意见或方案，列出对比
4. **趋势展望** - 未来发展方向和影响
5. **信息来源** - 列出主要参考来源（标题+链接）

请使用 Markdown 格式，层次分明，便于阅读。如果搜索材料不充分，请明确指出并建议进一步调研方向。`

	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("## 搜索材料\n\n%s\n\n## 调研主题\n\n%s\n\n请按照系统提示的要求，生成结构化的调研报告。", contextInfo, ctx.Content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	resp, err := client.Chat(messages, ai.WithTemperature(0.7), ai.WithMaxTokens(3000))
	if err != nil {
		return &AgentResult{
			Content: "调研过程遇到技术问题，请稍后重试。",
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

// ProcessStream 流式深度调研处理
func (ra *ResearchAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	client := ra.GetAIClient()
	if client == nil {
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

	systemPrompt := `你是一位资深技术研究员。请基于提供的搜索材料，对用户的问题进行深度分析和总结。
输出要求：核心观点 → 详细分析 → 观点对比 → 趋势展望 → 信息来源。使用 Markdown 格式。`

	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("## 搜索材料\n\n%s\n\n## 调研主题\n\n%s", contextInfo, ctx.Content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	return client.ChatStream(messages, func(chunk ai.StreamChunk) {
		if callback != nil {
			callback(chunk)
		}
	}, ai.WithTemperature(0.7), ai.WithMaxTokens(3000))
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

// deepResearch 深度调研：8 角度搜索 → 去重 → 取前 10 条抓取正文 → 汇总
func (ra *ResearchAgent) deepResearch(query string) ([]SearchResult, error) {
	var allResults []SearchResult

	// 使用当前年份而非硬编码，避免跨年后查询失效。
	// 8 轮多角度搜索，覆盖不同信息源和语言。
	currentYear := time.Now().Format("2006")
	queries := []string{
		query,
		query + " 最新进展 " + currentYear,
		query + " research paper site:arxiv.org",
		query + " technical deep dive analysis",
		query + " latest developments trends",
		query + " 技术分析 深度解读",
		query + " vs comparison alternatives",
		query + " future outlook predictions",
	}

	for i, q := range queries {
		if ra.webAgent != nil {
			results, err := ra.webAgent.search(q)
			if err == nil && len(results) > 0 {
				allResults = append(allResults, results...)
			}
		}
		// 递增延迟避免被限流
		if i < len(queries)-1 {
			time.Sleep(time.Duration(400+i*100) * time.Millisecond)
		}
	}

	// 去重：按链接
	seen := make(map[string]bool)
	unique := []SearchResult{}
	for _, r := range allResults {
		if r.Link == "" || seen[r.Link] {
			continue
		}
		seen[r.Link] = true
		unique = append(unique, r)
		if len(unique) >= 20 {
			break
		}
	}

	// 抓取前 5 条结果的正文摘要（丰富上下文）
	if ra.webAgent != nil && len(unique) > 0 {
		fetchCount := 5
		if len(unique) < fetchCount {
			fetchCount = len(unique)
		}
		for i := 0; i < fetchCount; i++ {
			content, err := ra.webAgent.FetchPageContent(unique[i].Link)
			if err == nil && len(content) > 200 {
				// 截取正文前 1500 字符补充到 snippet
				if len(content) > 1500 {
					content = content[:1500] + "..."
				}
				unique[i].Snippet = unique[i].Snippet + "\n\n[页面正文摘要]\n" + content
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	return unique, nil
}

// summarizeSearchResults 将搜索结果格式化为专业调研上下文
func (ra *ResearchAgent) summarizeSearchResults(results []SearchResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 调研结果（共 %d 条）\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("### %d. %s\n", i+1, result.Title))
		if result.Snippet != "" {
			// 限制每条摘要长度，保持可读性
			snippet := result.Snippet
			if len(snippet) > 800 {
				snippet = snippet[:800] + "..."
			}
			sb.WriteString(fmt.Sprintf("%s\n", snippet))
		}
		if result.Link != "" {
			sb.WriteString(fmt.Sprintf("> 来源: [%s](%s)\n", result.Link, result.Link))
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
