package ai

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Provider AI 提供商配置
type Provider struct {
	Name    string
	BaseURL string
	Model   string
}

// 预置的 provider 列表
// PRD 9.2：默认 DeepSeek，备选智谱 AI（GLM-4-Flash）和通义千问（Qwen-Turbo）
var Providers = map[string]Provider{
	"deepseek": {
		Name:    "DeepSeek",
		BaseURL: "https://api.deepseek.com",
		Model:   "deepseek-chat",
	},
	"zhipu": {
		Name:    "智谱 AI (GLM-4-Flash)",
		BaseURL: "https://open.bigmodel.cn/api/paas/v4",
		Model:   "glm-4-flash",
	},
	"qwen": {
		Name:    "通义千问 (Qwen-Turbo)",
		BaseURL: "https://dashscope.aliyuncs.com/compatible-mode/v1",
		Model:   "qwen-turbo",
	},
}

// Message 聊天消息
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest 聊天请求（OpenAI 兼容格式）
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

// ChatResponse 聊天响应
type ChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// Client AI API 客户端
// 支持多个 provider，遵循 OpenAI 兼容的 Chat Completions 接口
type Client struct {
	apiKey      string
	provider    string
	providerCfg Provider
	baseURL     string
	model       string
	httpClient  *http.Client
}

// NewClient 创建 AI 客户端
// provider 可选: deepseek / zhipu / qwen
func NewClient(provider, apiKey string) *Client {
	cfg, ok := Providers[provider]
	if !ok {
		cfg = Providers["deepseek"]
		provider = "deepseek"
	}
	return &Client{
		apiKey:      apiKey,
		provider:    provider,
		providerCfg: cfg,
		baseURL:     cfg.BaseURL,
		model:       cfg.Model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// SetAPIKey 更新 API Key
func (c *Client) SetAPIKey(apiKey string) {
	c.apiKey = apiKey
}

// SetProvider 切换 provider
func (c *Client) SetProvider(provider string) {
	cfg, ok := Providers[provider]
	if !ok {
		return
	}
	c.provider = provider
	c.providerCfg = cfg
	c.baseURL = cfg.BaseURL
	c.model = cfg.Model
}

// GetProvider 获取当前 provider 名称
func (c *Client) GetProvider() string {
	return c.provider
}

// Close 关闭客户端
func (c *Client) Close() {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

// Chat 发送聊天请求
func (c *Client) Chat(messages []Message, opts ...func(*ChatRequest)) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API Key 未配置")
	}

	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.8,
		MaxTokens:   2048,
	}

	for _, opt := range opts {
		opt(&req)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	url := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var chatResp ChatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("API 返回空响应")
	}

	return chatResp.Choices[0].Message.Content, nil
}

// WithTemperature 设置温度
func WithTemperature(temp float64) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.Temperature = temp
	}
}

// WithMaxTokens 设置最大 token 数
func WithMaxTokens(tokens int) func(*ChatRequest) {
	return func(r *ChatRequest) {
		r.MaxTokens = tokens
	}
}

// StreamChunk 流式响应的片段
type StreamChunk struct {
	Content      string `json:"content"`
	Done         bool   `json:"done"`
	Error        string `json:"error,omitempty"`
	FinishReason string `json:"finish_reason,omitempty"`
}

// ChatStream 发送流式聊天请求（SSE）
// onChunk 每收到一个片段就调用，用于实时推送
func (c *Client) ChatStream(messages []Message, onChunk func(StreamChunk), opts ...func(*ChatRequest)) error {
	if c.apiKey == "" {
		return fmt.Errorf("API Key 未配置")
	}

	req := ChatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.8,
		MaxTokens:   2048,
		Stream:      true, // 启用流式模式
	}

	for _, opt := range opts {
		opt(&req)
	}

	reqBody, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("序列化请求失败: %w", err)
	}

	url := strings.TrimRight(c.baseURL, "/") + "/chat/completions"
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API 返回错误 (HTTP %d): %s", resp.StatusCode, string(body))
	}

	// 解析 SSE 流
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// 流结束，发送 Done 标记
				onChunk(StreamChunk{Done: true})
				break
			}
			return fmt.Errorf("读取流失败: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}

		data := strings.TrimPrefix(line, "data:")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			onChunk(StreamChunk{Done: true})
			break
		}

		// 解析 JSON 片段
		var streamResp struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			// 解析失败，跳过该片段
			continue
		}

		if len(streamResp.Choices) > 0 {
			chunk := StreamChunk{
				Content:      streamResp.Choices[0].Delta.Content,
				FinishReason: streamResp.Choices[0].FinishReason,
			}
			onChunk(chunk)
		}
	}

	return nil
}
