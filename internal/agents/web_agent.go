package agents

import (
	"ai-companion/internal/ai"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type SearchProvider interface {
	Name() string
	Search(query string) ([]SearchResult, error)
}

type DuckDuckGoProvider struct {
	httpClient *http.Client
}

func NewDuckDuckGoProvider() *DuckDuckGoProvider {
	return &DuckDuckGoProvider{
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

func (p *DuckDuckGoProvider) Name() string { return "duckduckgo" }

func extractTitle(text string) string {
	if idx := strings.Index(text, " - "); idx != -1 {
		return text[:idx]
	}
	if idx := strings.Index(text, " — "); idx != -1 {
		return text[:idx]
	}
	if len(text) > 60 {
		return text[:60] + "..."
	}
	return text
}

func (p *DuckDuckGoProvider) Search(query string) ([]SearchResult, error) {
	apiURL := fmt.Sprintf("https://api.duckduckgo.com/?q=%s&format=json&no_html=1&skip_disambig=1", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := p.httpClient.Do(req)
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

	var ddgResp struct {
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		Heading       string `json:"Heading"`
		RelatedTopics []struct {
			Text     string `json:"Text"`
			FirstURL string `json:"FirstURL"`
		} `json:"RelatedTopics"`
		Results []struct {
			Title    string `json:"Title"`
			FirstURL string `json:"FirstURL"`
			Text     string `json:"Text"`
		} `json:"Results"`
	}

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
		if topic.Text != "" && len(results) < 10 {
			results = append(results, SearchResult{
				Title:   extractTitle(topic.Text),
				Link:    topic.FirstURL,
				Snippet: topic.Text,
			})
		}
	}

	for _, result := range ddgResp.Results {
		if result.Title != "" && len(results) < 10 {
			results = append(results, SearchResult{
				Title:   result.Title,
				Link:    result.FirstURL,
				Snippet: result.Text,
			})
		}
	}

	return results, nil
}

type BingProvider struct {
	httpClient *http.Client
	apiKey     string
}

func NewBingProvider(apiKey string) *BingProvider {
	return &BingProvider{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		apiKey:     apiKey,
	}
}

func (p *BingProvider) Name() string { return "bing" }

func (p *BingProvider) Search(query string) ([]SearchResult, error) {
	if p.apiKey == "" {
		return nil, fmt.Errorf("Bing API Key 未配置")
	}

	apiURL := fmt.Sprintf("https://api.bing.microsoft.com/v7.0/search?q=%s", url.QueryEscape(query))
	req, err := http.NewRequestWithContext(context.Background(), "GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", p.apiKey)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Bing 搜索失败: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var bingResp struct {
		WebPages struct {
			Value []struct {
				Name        string `json:"name"`
				URL         string `json:"url"`
				Snippet     string `json:"snippet"`
				Description string `json:"description"`
			} `json:"value"`
		} `json:"webPages"`
	}

	if err := json.Unmarshal(body, &bingResp); err != nil {
		return nil, err
	}

	var results []SearchResult
	for _, item := range bingResp.WebPages.Value {
		if item.Name != "" && len(results) < 10 {
			snippet := item.Snippet
			if snippet == "" {
				snippet = item.Description
			}
			results = append(results, SearchResult{
				Title:   item.Name,
				Link:    item.URL,
				Snippet: snippet,
			})
		}
	}

	return results, nil
}

type WebAgent struct {
	BaseAgent
	httpClient      *http.Client
	searchProviders []SearchProvider
	currentProvider string
	bingAPIKey      string
}

func NewWebAgent(aiClient *ai.Client) *WebAgent {
	wa := &WebAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "web",
			desc:     "联网搜索：网络搜索、信息查询、网页内容抓取",
		},
		httpClient:      &http.Client{Timeout: 30 * time.Second},
		searchProviders: make([]SearchProvider, 0),
		currentProvider: "duckduckgo",
	}
	wa.searchProviders = append(wa.searchProviders, NewDuckDuckGoProvider())
	return wa
}

func (wa *WebAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "web", Description: "联网搜索、网页内容抓取", InputDesc: "搜索关键词或URL", OutputDesc: "搜索结果和网页内容"},
	}
}

func (wa *WebAgent) SetBingAPIKey(apiKey string) {
	wa.bingAPIKey = apiKey
	for i, provider := range wa.searchProviders {
		if provider.Name() == "bing" {
			wa.searchProviders[i] = NewBingProvider(apiKey)
			return
		}
	}
	wa.searchProviders = append(wa.searchProviders, NewBingProvider(apiKey))
}

func (wa *WebAgent) SetSearchProvider(name string) {
	for _, provider := range wa.searchProviders {
		if provider.Name() == name {
			wa.currentProvider = name
			return
		}
	}
}

func (wa *WebAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"搜索", "查一下", "了解一下", "调研", "研究", "什么是", "怎么",
		"为什么", "如何", "最新", "新闻", "信息", "资料", "百度", "谷歌",
		"bing", "必应", "网页", "网站", "链接", "下载", "天气", "汇率",
	}
	return KeywordMatch(ctx.Content, keywords)
}

func (wa *WebAgent) Process(ctx AgentContext) (*AgentResult, error) {
	client := wa.GetAIClient()
	if client == nil {
		return &AgentResult{
			Content: "我可以帮你搜索网络信息。你想了解什么？",
			Emotion: "认真",
		}, nil
	}

	searchResults, searchErr := wa.search(ctx.Content)
	var contextInfo string
	var providerInfo string

	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = wa.summarizeSearchResults(searchResults)
		providerInfo = fmt.Sprintf("（使用 %s 搜索）", wa.getProviderDisplayName())
	} else {
		contextInfo = "（网络搜索暂时不可用，我将基于我的知识来回答）"
		if searchErr != nil {
			contextInfo += fmt.Sprintf(" 错误: %v", searchErr)
		}
	}

	systemPrompt := ai.BuildSystemPrompt("research", "")
	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索上下文：\n%s\n%s\n\n用户问题：%s", contextInfo, providerInfo, ctx.Content)
	}

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
	}
	messages = append(messages, ctx.History...)
	messages = append(messages, ai.Message{Role: "user", Content: userMessage})

	resp, err := client.Chat(messages, ai.WithTemperature(0.7))
	if err != nil {
		return &AgentResult{
			Content: "网络搜索出了点小问题，不过我们可以继续聊聊其他话题。",
			Emotion: "关心",
		}, nil
	}

	return &AgentResult{
		Content:      resp,
		Emotion:      "认真",
		ShouldRecord: true,
		Data:         searchResults,
	}, nil
}

func (wa *WebAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	client := wa.GetAIClient()
	if client == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我可以帮你搜索网络信息。你想了解什么？", Done: true})
		}
		return nil
	}

	searchResults, searchErr := wa.search(ctx.Content)
	var contextInfo string
	var providerInfo string

	if searchErr == nil && len(searchResults) > 0 {
		contextInfo = wa.summarizeSearchResults(searchResults)
		providerInfo = fmt.Sprintf("（使用 %s 搜索）", wa.getProviderDisplayName())
	} else {
		contextInfo = "（网络搜索暂时不可用，我将基于我的知识来回答）"
	}

	systemPrompt := ai.BuildSystemPrompt("research", "")
	userMessage := ctx.Content
	if contextInfo != "" {
		userMessage = fmt.Sprintf("搜索上下文：\n%s\n%s\n\n用户问题：%s", contextInfo, providerInfo, ctx.Content)
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
	}, ai.WithTemperature(0.7))
}

// search 统一搜索入口 + 失败回退
// 行为：
//  1. 优先用 currentProvider 搜索；
//  2. 失败（error）或返回 0 条结果时，按注册顺序回退到其他 provider；
//  3. 全部失败时返回最后一次的 error。
// 这样无论默认是 bing 还是 duckduckgo，都能双向回退，
// 避免出现"duckduckgo 失败但没回退到 bing"导致的搜索质量退化。
func (wa *WebAgent) search(query string) ([]SearchResult, error) {
	// 按"当前 provider 优先 + 其它 provider 兜底"的顺序排列尝试列表
	ordered := make([]SearchProvider, 0, len(wa.searchProviders))
	seen := make(map[string]bool)
	for _, p := range wa.searchProviders {
		if p.Name() == wa.currentProvider {
			ordered = append(ordered, p)
			seen[p.Name()] = true
			break
		}
	}
	for _, p := range wa.searchProviders {
		if !seen[p.Name()] {
			ordered = append(ordered, p)
			seen[p.Name()] = true
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("未配置搜索源")
	}

	var lastErr error
	for _, p := range ordered {
		results, err := p.Search(query)
		if err == nil && len(results) > 0 {
			return results, nil
		}
		if err != nil {
			lastErr = err
		}
		// err == nil 但结果为空：记下空结果但继续尝试下一个
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, nil
}

func (wa *WebAgent) getProviderDisplayName() string {
	switch wa.currentProvider {
	case "duckduckgo":
		return "DuckDuckGo"
	case "bing":
		return "Bing"
	default:
		return wa.currentProvider
	}
}

// isPrivateHost 判断解析后的 URL 是否指向不应被访问的内网/SSRF 目标
// 覆盖：IPv4 私网/回环/链路本地（含云元数据 169.254.169.254）、IPv6 私网、
// 域名形式的 loopback / localhost / 常见内网后缀。
func isPrivateHost(parsedURL *url.URL) bool {
	host := parsedURL.Hostname()
	if host == "" {
		return true
	}

	lower := strings.ToLower(host)

	// 1) 字面主机名（loopback、内网后缀、特殊保留名）
	switch lower {
	case "localhost", "ip6-localhost", "ip6-loopback", "metadata.google.internal":
		return true
	}
	if strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".local") ||
		strings.HasSuffix(lower, ".internal") ||
		strings.HasSuffix(lower, ".intranet") {
		return true
	}

	// 2) 解析为 IP 再判断
	if ip := net.ParseIP(host); ip != nil {
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
			ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() {
			return true
		}
		// 显式 169.254.0.0/16（云元数据地址）
		if ip4 := ip.To4(); ip4 != nil {
			if ip4[0] == 169 && ip4[1] == 254 {
				return true
			}
		}
		return false
	}

	// 3) 域名：尝试 DNS 解析为 IP，再判断
	if ips, err := net.LookupIP(host); err == nil {
		for _, ip := range ips {
			if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate() ||
				ip.IsUnspecified() || ip.IsMulticast() {
				return true
			}
			if ip4 := ip.To4(); ip4 != nil {
				if ip4[0] == 169 && ip4[1] == 254 {
					return true
				}
			}
		}
	}

	return false
}

// FetchPageContent 抓取网页正文
// 安全加固：限制响应体大小、拒绝内网/SSRF 目标、强制 https 优先。
func (wa *WebAgent) FetchPageContent(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	if parsedURL.Scheme == "" {
		urlStr = "https://" + urlStr
		parsedURL, err = url.Parse(urlStr)
		if err != nil {
			return "", err
		}
	}

	// 仅允许 http/https scheme
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("不支持的协议: %s", parsedURL.Scheme)
	}

	// SSRF 防护：拒绝内网/云元数据地址
	if isPrivateHost(parsedURL) {
		return "", fmt.Errorf("禁止访问内网/本机/云元数据地址: %s", parsedURL.Host)
	}

	req, err := http.NewRequestWithContext(context.Background(), "GET", urlStr, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := wa.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("请求失败: %d", resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/json") {
		return "", fmt.Errorf("不支持的内容类型: %s", contentType)
	}

	// 限制响应体大小（默认 2MB），防止撑爆内存。
	// 优先使用 Content-Length 提前拒绝；fallback 用 io.LimitReader 兜底。
	const maxBodyBytes = 2 * 1024 * 1024
	if resp.ContentLength > 0 && resp.ContentLength > maxBodyBytes {
		return "", fmt.Errorf("响应体过大: %d 字节", resp.ContentLength)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return "", err
	}
	if len(body) > maxBodyBytes {
		return "", fmt.Errorf("响应体超过限制 (%d > %d 字节)", len(body), maxBodyBytes)
	}

	return wa.extractTextFromHTML(string(body)), nil
}

func (wa *WebAgent) extractTextFromHTML(html string) string {
	html = strings.ReplaceAll(html, "\n", " ")
	html = strings.ReplaceAll(html, "\r", " ")

	re := regexp.MustCompile(`<script[^>]*>.*?</script>`)
	html = re.ReplaceAllString(html, "")

	re = regexp.MustCompile(`<style[^>]*>.*?</style>`)
	html = re.ReplaceAllString(html, "")

	re = regexp.MustCompile(`<[^>]+>`)
	text := re.ReplaceAllString(html, " ")

	re = regexp.MustCompile(`\s+`)
	text = re.ReplaceAllString(text, " ")

	text = strings.TrimSpace(text)

	if len(text) > 3000 {
		text = text[:3000] + "..."
	}

	return text
}

func (wa *WebAgent) summarizeSearchResults(results []SearchResult) string {
	var sb strings.Builder
	sb.WriteString("以下是搜索到的相关信息：\n\n")

	for i, result := range results {
		if i >= 8 {
			break
		}
		if result.Title != "" {
			sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))
		}
		if result.Snippet != "" {
			snippet := result.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
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
