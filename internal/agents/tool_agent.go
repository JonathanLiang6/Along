package agents

import (
	"ai-companion/internal/ai"
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ToolAgent 工具操作 Agent
type ToolAgent struct {
	aiClient *ai.Client
}

// NewToolAgent 创建工具 Agent
func NewToolAgent(client *ai.Client) *ToolAgent {
	return &ToolAgent{aiClient: client}
}

// ToolRequest 工具请求
type ToolRequest struct {
	Action string                 `json:"action"`
	Params map[string]interface{} `json:"params"`
}

// ToolResponse 工具响应
type ToolResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data"`
	Error   string      `json:"error,omitempty"`
}

// Handle 处理工具调用（返回字符串）
func (ta *ToolAgent) Handle(content string) ToolResponse {
	// 解析请求
	req, err := ta.parseRequest(content)
	if err != nil {
		return ToolResponse{Success: false, Error: err.Error()}
	}

	switch req.Action {
	case "read_file":
		return ta.ReadFile(ta.getStringParam(req.Params, "path"))
	case "write_file":
		return ta.WriteFile(
			ta.getStringParam(req.Params, "path"),
			ta.getStringParam(req.Params, "content"),
		)
	case "list_dir":
		return ta.ListDir(ta.getStringParam(req.Params, "path"))
	case "git_status":
		return ta.GitStatus(ta.getStringParam(req.Params, "repo_path"))
	case "git_log":
		limit := ta.getIntParam(req.Params, "limit", 10)
		return ta.GitLog(
			ta.getStringParam(req.Params, "repo_path"),
			limit,
		)
	case "open_browser":
		return ta.OpenBrowser(ta.getStringParam(req.Params, "url"))
	}

	return ToolResponse{Success: false, Error: fmt.Sprintf("未知工具操作: %s", req.Action)}
}

// ProcessLegacy 处理工具调用（旧版兼容）
func (ta *ToolAgent) ProcessLegacy(content string) (*AgentResponse, error) {
	resp := ta.Handle(content)

	emotion := "平静"
	if !resp.Success {
		emotion = "困扰"
	}

	return &AgentResponse{
		Content: ta.formatResponse(resp),
		Emotion: emotion,
		Data:    resp,
	}, nil
}

// parseRequest 解析工具请求
func (ta *ToolAgent) parseRequest(content string) (*ToolRequest, error) {
	req := &ToolRequest{
		Params: make(map[string]interface{}),
	}

	// 从内容中提取操作类型
	content = strings.TrimSpace(content)
	lower := strings.ToLower(content)

	// 检测操作类型
	if strings.Contains(lower, "读取文件") || strings.Contains(lower, "read file") {
		req.Action = "read_file"
		req.Params["path"] = ta.extractPath(content)
	} else if strings.Contains(lower, "写入文件") || strings.Contains(lower, "write file") {
		req.Action = "write_file"
		parts := ta.extractWriteParams(content)
		req.Params["path"] = parts["path"]
		req.Params["content"] = parts["content"]
	} else if strings.Contains(lower, "列出目录") || strings.Contains(lower, "list dir") {
		req.Action = "list_dir"
		req.Params["path"] = ta.extractPath(content)
	} else if strings.Contains(lower, "git状态") || strings.Contains(lower, "git status") {
		req.Action = "git_status"
		req.Params["repo_path"] = ta.extractPath(content)
	} else if strings.Contains(lower, "git日志") || strings.Contains(lower, "git log") {
		req.Action = "git_log"
		req.Params["repo_path"] = ta.extractPath(content)
		req.Params["limit"] = ta.extractLimit(content)
	} else if strings.Contains(lower, "打开链接") || strings.Contains(lower, "open browser") {
		req.Action = "open_browser"
		req.Params["url"] = ta.extractURL(content)
	} else {
		return nil, fmt.Errorf("无法识别的工具操作")
	}

	return req, nil
}

// ReadFile 读取文件内容
func (ta *ToolAgent) ReadFile(path string) ToolResponse {
	if path == "" {
		return ToolResponse{Success: false, Error: "文件路径不能为空"}
	}

	// 转换路径
	path = ta.normalizePath(path)

	// 检查文件是否存在
	info, err := os.Stat(path)
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("文件不存在或无法访问: %v", err)}
	}

	if info.IsDir() {
		return ToolResponse{Success: false, Error: "指定路径是目录，不是文件"}
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("读取文件失败: %v", err)}
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"path":    path,
			"content": string(content),
			"size":    len(content),
		},
	}
}

// WriteFile 写入文件
func (ta *ToolAgent) WriteFile(path, content string) ToolResponse {
	if path == "" {
		return ToolResponse{Success: false, Error: "文件路径不能为空"}
	}

	// 转换路径
	path = ta.normalizePath(path)

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("创建目录失败: %v", err)}
	}

	// 写入文件
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("写入文件失败: %v", err)}
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"path":    path,
			"size":    len(content),
			"message": "文件写入成功",
		},
	}
}

// ListDir 列出目录内容
func (ta *ToolAgent) ListDir(path string) ToolResponse {
	if path == "" {
		return ToolResponse{Success: false, Error: "目录路径不能为空"}
	}

	// 转换路径
	path = ta.normalizePath(path)

	// 检查目录是否存在
	info, err := os.Stat(path)
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("目录不存在或无法访问: %v", err)}
	}

	if !info.IsDir() {
		return ToolResponse{Success: false, Error: "指定路径是文件，不是目录"}
	}

	// 读取目录内容
	entries, err := os.ReadDir(path)
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("读取目录失败: %v", err)}
	}

	// 构建目录列表
	var items []map[string]interface{}
	for _, entry := range entries {
		item := map[string]interface{}{
			"name":  entry.Name(),
			"isDir": entry.IsDir(),
		}

		info, err := entry.Info()
		if err == nil {
			item["size"] = info.Size()
			item["modTime"] = info.ModTime().Format("2006-01-02 15:04:05")
		}

		items = append(items, item)
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"path":  path,
			"items": items,
			"count": len(items),
		},
	}
}

// GitStatus 获取git状态
func (ta *ToolAgent) GitStatus(repoPath string) ToolResponse {
	if repoPath == "" {
		repoPath = "." // 默认当前目录
	}

	repoPath = ta.normalizePath(repoPath)

	// 检查是否是git仓库
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return ToolResponse{Success: false, Error: "指定路径不是git仓库"}
	}

	// 执行git status命令
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("执行git status失败: %v", err)}
	}

	// 解析输出
	var changes []map[string]string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) >= 3 {
			status := strings.TrimSpace(line[:2])
			file := line[3:]
			changes = append(changes, map[string]string{
				"status": status,
				"file":   file,
			})
		}
	}

	// 获取分支名
	branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = repoPath
	branchOutput, err := branchCmd.Output()
	branch := "unknown"
	if err == nil {
		branch = strings.TrimSpace(string(branchOutput))
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"repoPath": repoPath,
			"branch":   branch,
			"changes":  changes,
			"clean":    len(changes) == 0,
		},
	}
}

// GitLog 获取git提交记录
func (ta *ToolAgent) GitLog(repoPath string, limit int) ToolResponse {
	if repoPath == "" {
		repoPath = "."
	}

	repoPath = ta.normalizePath(repoPath)

	// 检查是否是git仓库
	gitDir := filepath.Join(repoPath, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return ToolResponse{Success: false, Error: "指定路径不是git仓库"}
	}

	if limit <= 0 {
		limit = 10
	}

	// 执行git log命令
	cmd := exec.Command("git", "log", fmt.Sprintf("-n%d", limit), "--pretty=format:%H|%h|%an|%ae|%ad|%s", "--date=iso")
	cmd.Dir = repoPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("执行git log失败: %v", err)}
	}

	// 解析输出
	var commits []map[string]string
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "|", 6)
		if len(parts) == 6 {
			commits = append(commits, map[string]string{
				"hash":    parts[0],
				"short":   parts[1],
				"author":  parts[2],
				"email":   parts[3],
				"date":    parts[4],
				"message": parts[5],
			})
		}
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"repoPath": repoPath,
			"commits":  commits,
			"count":    len(commits),
		},
	}
}

// OpenBrowser 打开浏览器链接
func (ta *ToolAgent) OpenBrowser(url string) ToolResponse {
	if url == "" {
		return ToolResponse{Success: false, Error: "URL不能为空"}
	}

	// 确保URL有协议
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "https://" + url
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default: // linux
		cmd = exec.Command("xdg-open", url)
	}

	if err := cmd.Start(); err != nil {
		return ToolResponse{Success: false, Error: fmt.Sprintf("打开浏览器失败: %v", err)}
	}

	return ToolResponse{
		Success: true,
		Data: map[string]interface{}{
			"url":     url,
			"message": "已在浏览器中打开链接",
		},
	}
}

// 辅助方法

func (ta *ToolAgent) normalizePath(path string) string {
	// 处理路径分隔符
	path = strings.ReplaceAll(path, "/", string(filepath.Separator))
	path = strings.ReplaceAll(path, "\\", string(filepath.Separator))

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}

	return absPath
}

func (ta *ToolAgent) getStringParam(params map[string]interface{}, key string) string {
	if val, ok := params[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

func (ta *ToolAgent) getIntParam(params map[string]interface{}, key string, defaultVal int) int {
	if val, ok := params[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		case string:
			var intVal int
			fmt.Sscanf(v, "%d", &intVal)
			return intVal
		}
	}
	return defaultVal
}

func (ta *ToolAgent) extractPath(content string) string {
	// 提取引号中的路径
	quotes := []string{"\"", "'", "「", "」", "【", "】", "《", "》"}
	for i := 0; i < len(quotes); i += 2 {
		start := strings.Index(content, quotes[i])
		if start != -1 {
			end := strings.Index(content[start+1:], quotes[i+1])
			if end != -1 {
				return content[start+1 : start+1+end]
			}
		}
	}

	// 尝试提取路径格式的字符串
	words := strings.Fields(content)
	for _, word := range words {
		if strings.Contains(word, ":") || strings.Contains(word, "/") || strings.Contains(word, "\\") {
			// 清理路径
			word = strings.Trim(word, ".,!?;:\"'")
			return word
		}
	}

	return ""
}

func (ta *ToolAgent) extractWriteParams(content string) map[string]string {
	result := map[string]string{
		"path":    "",
		"content": "",
	}

	// 提取路径
	result["path"] = ta.extractPath(content)

	// 尝试提取内容（在"内容"、"写入"等关键词之后）
	keywords := []string{"内容：", "内容:", "写入：", "写入:", "内容是"}
	for _, kw := range keywords {
		if idx := strings.Index(content, kw); idx != -1 {
			result["content"] = strings.TrimSpace(content[idx+len(kw):])
			break
		}
	}

	return result
}

func (ta *ToolAgent) extractLimit(content string) int {
	// 提取数字
	var limit int
	fmt.Sscanf(content, "%d", &limit)
	if limit <= 0 {
		limit = 10
	}
	return limit
}

func (ta *ToolAgent) extractURL(content string) string {
	// 提取URL
	urlPatterns := []string{"http://", "https://", "www."}
	for _, pattern := range urlPatterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			url := content[idx:]
			// 截取到空白字符
			for i, c := range url {
				if c == ' ' || c == '\t' || c == '\n' {
					return url[:i]
				}
			}
			return url
		}
	}
	return ""
}

func (ta *ToolAgent) formatResponse(resp ToolResponse) string {
	if !resp.Success {
		return fmt.Sprintf("操作失败: %s", resp.Error)
	}

	switch data := resp.Data.(type) {
	case map[string]interface{}:
		if msg, ok := data["message"].(string); ok {
			return msg
		}
		if _, ok := data["content"].(string); ok {
			// 文件读取
			return fmt.Sprintf("成功读取文件，大小: %d 字节", data["size"])
		}
		if _, ok := data["items"].([]map[string]interface{}); ok {
			// 目录列表
			return fmt.Sprintf("目录包含 %d 个项目", data["count"])
		}
		if _, ok := data["changes"].([]map[string]string); ok {
			// Git状态
			if data["clean"].(bool) {
				return fmt.Sprintf("Git仓库状态干净，当前分支: %s", data["branch"])
			}
			return fmt.Sprintf("Git仓库有 %d 个变更，当前分支: %s", len(data["changes"].([]map[string]string)), data["branch"])
		}
		if _, ok := data["commits"].([]map[string]string); ok {
			// Git日志
			return fmt.Sprintf("获取到 %d 条提交记录", data["count"])
		}
	}

	return "操作成功"
}

// ==================== Agent 接口实现 ====================

// Name 返回 Agent 名称
func (ta *ToolAgent) Name() string {
	return "tool"
}

// Description 返回 Agent 描述
func (ta *ToolAgent) Description() string {
	return "工具操作 Agent，负责文件读写、目录浏览、Git 操作、打开浏览器等工具调用"
}

// Match 计算匹配度
func (ta *ToolAgent) Match(ctx AgentContext) float64 {
	content := strings.ToLower(ctx.Content)
	keywords := []string{
		"读取文件", "read file", "写入文件", "write file",
		"列出目录", "list dir", "看一下目录", "打开文件",
		"git状态", "git status", "git日志", "git log",
		"打开链接", "open browser", "帮我打开",
	}
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			return 0.9
		}
	}
	return 0.1
}

// Process 同步处理
func (ta *ToolAgent) Process(ctx AgentContext) (*AgentResult, error) {
	resp := ta.Handle(ctx.Content)

	emotion := "平静"
	if !resp.Success {
		emotion = "困扰"
	}

	return &AgentResult{
		Content: ta.formatResponse(resp),
		Emotion: emotion,
		Data:    resp,
	}, nil
}

// ProcessStream 流式处理（工具调用不支持流式，直接返回结果）
func (ta *ToolAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	result, err := ta.Process(ctx)
	if err != nil {
		callback(ai.StreamChunk{Error: err.Error(), Done: true})
		return err
	}

	callback(ai.StreamChunk{Content: result.Content, Done: true})
	return nil
}

// UpdateAIClient 更新 AI 客户端
func (ta *ToolAgent) UpdateAIClient(client *ai.Client) {
	ta.aiClient = client
}

// Capabilities 能力声明
func (ta *ToolAgent) Capabilities() []Capability {
	return []Capability{
		{Name: "tool", Description: "文件读写、目录浏览、Git操作、浏览器打开", InputDesc: "操作类型和参数", OutputDesc: "操作结果"},
	}
}
