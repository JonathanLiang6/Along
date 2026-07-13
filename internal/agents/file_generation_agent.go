package agents

import (
	"ai-companion/internal/ai"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// 文档模板类型
const (
	TemplateResearch   = "research"   // 研究报告
	TemplateWeekly     = "weekly"     // 周报
	TemplateMeeting    = "meeting"    // 会议纪要
	TemplateTechReview = "tech_review" // 技术评测
	TemplateGeneral    = "general"    // 通用
)

// FileGenerationAgent 文件生成 Agent
// 职责：接收结构化内容，按照标准模板生成 markdown 文件并保存
// 不负责信息搜索和整合，只负责文档格式化和文件写入
type FileGenerationAgent struct {
	BaseAgent
	outputDir string
}

// NewFileGenerationAgent 创建文件生成 Agent
func NewFileGenerationAgent(aiClient *ai.Client) *FileGenerationAgent {
	return &FileGenerationAgent{
		BaseAgent: BaseAgent{
			aiClient: aiClient,
			name:     "file_generation",
			desc:     "文件生成：按照标准模板将内容格式化为markdown文档并保存到文件",
		},
	}
}

// SetOutputDir 设置输出目录
func (fa *FileGenerationAgent) SetOutputDir(dir string) {
	fa.outputDir = dir
}

// Match 计算匹配度
func (fa *FileGenerationAgent) Match(ctx AgentContext) float64 {
	keywords := []string{
		"生成文档", "生成报告", "保存文档", "导出文档",
		"写文件", "生成markdown", "生成md",
		"周报文档", "报告文档", "文档模板",
	}
	return KeywordMatch(ctx.Content, keywords)
}

// Process 同步处理
func (fa *FileGenerationAgent) Process(ctx AgentContext) (*AgentResult, error) {
	if fa.aiClient == nil {
		return &AgentResult{
			Content: "我可以帮你生成格式化的markdown文档。请提供需要格式化的内容。",
			Emotion: "认真",
		}, nil
	}

	// 获取输入内容：优先从 Extra 获取（工作流场景），否则用 ctx.Content
	content := ctx.Content
	if rawContent, ok := ctx.Extra["raw_content"].(string); ok && rawContent != "" {
		content = rawContent
	}

	if strings.TrimSpace(content) == "" {
		return &AgentResult{
			Content: "内容为空，无法生成文档。请先提供需要格式化的内容。",
			Emotion: "认真",
		}, nil
	}

	// 文档标题
	title := ""
	if t, ok := ctx.Extra["title"].(string); ok && t != "" {
		title = t
	}

	// 文档模板类型
	template := TemplateGeneral
	if tpl, ok := ctx.Extra["template"].(string); ok && tpl != "" {
		template = tpl
	}

	// 生成格式化文档
	document, err := fa.formatDocument(content, title, template)
	if err != nil {
		return &AgentResult{
			Content: fmt.Sprintf("生成文档时遇到问题：%v", err),
			Emotion: "抱歉",
		}, nil
	}

	// 保存文件
	var filePath string
	if fa.outputDir != "" {
		fileTitle := title
		if fileTitle == "" {
			fileTitle = ctx.Content
			if len(fileTitle) > 20 {
				fileTitle = fileTitle[:20]
			}
		}
		filePath = fa.saveDocument(fileTitle, document)
	}

	// 构建用户友好的回复
	reply := document
	if filePath != "" {
		reply = fmt.Sprintf("%s\n\n---\n📄 文档已保存到：`%s`", document, filePath)
	}

	return &AgentResult{
		Content:      reply,
		Emotion:      "认真",
		ShouldRecord: true,
		Data: map[string]interface{}{
			"file_path": filePath,
			"template":  template,
			"file_size": len(document),
		},
	}, nil
}

// ProcessStream 流式处理
func (fa *FileGenerationAgent) ProcessStream(ctx AgentContext, callback StreamCallback) error {
	if fa.aiClient == nil {
		if callback != nil {
			callback(ai.StreamChunk{Content: "我可以帮你生成格式化的markdown文档。请提供需要格式化的内容。", Done: true})
		}
		return nil
	}

	result, err := fa.Process(ctx)
	if err != nil {
		return err
	}

	if callback != nil {
		callback(ai.StreamChunk{Content: result.Content, Done: true})
	}
	return nil
}

// formatDocument 调用 AI 将内容格式化为标准文档
func (fa *FileGenerationAgent) formatDocument(rawContent, title, template string) (string, error) {
	if fa.aiClient == nil {
		return "", fmt.Errorf("AI客户端未初始化")
	}

	if title == "" {
		title = "未命名文档"
	}

	systemPrompt := ai.BuildSystemPrompt("file_generation", "")

	templateGuide := fa.getTemplateGuide(template)
	dateStr := time.Now().Format("2006-01-02")

	userMessage := fmt.Sprintf(`请将以下内容格式化为标准的markdown文档。

文档标题：%s
日期：%s
模板类型：%s

%s

原始内容：
%s

要求：
- 严格按照模板结构组织内容
- 保持原始信息的准确性，不添加臆测
- 使用规范的markdown语法（标题层级、列表、粗体、引用等）
- 语言：中文
- 在文档末尾添加「生成信息」部分，包含生成时间和字数统计`, title, dateStr, template, templateGuide, rawContent)

	messages := []ai.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userMessage},
	}

	resp, err := fa.aiClient.Chat(messages, ai.WithTemperature(0.4), ai.WithMaxTokens(3000))
	if err != nil {
		return "", err
	}

	return resp, nil
}

// getTemplateGuide 根据模板类型返回结构指南
func (fa *FileGenerationAgent) getTemplateGuide(template string) string {
	switch template {
	case TemplateResearch:
		return `请按以下结构生成研究报告：
# {标题} - 研究报告 ({日期})

## 摘要
（3-5句话概括核心发现）

## 主要趋势
（分点列出主要发展趋势）

## 技术突破
（具体技术进展及说明）

## 代表项目/论文
（列出重要项目或论文，附简要说明）

## 应用案例
（实际应用场景）

## 未来展望
（趋势预测和建议）

## 参考资料
（列出信息来源）

---
> 生成时间：{时间} | 字数：{字数}`

	case TemplateWeekly:
		return `请按以下结构生成周报：
# {标题} - 周报 ({日期})

## 本周概要
（2-3句话总结本周重点）

## 完成事项
（列出本周完成的工作）

## 进行中事项
（列出正在进行的工作及进度）

## 问题与风险
（遇到的问题和潜在风险）

## 下周计划
（下周重点方向）

## 学习与思考
（本周的收获和思考）

---
> 生成时间：{时间} | 字数：{字数}`

	case TemplateMeeting:
		return `请按以下结构生成会议纪要：
# {标题} - 会议纪要 ({日期})

## 会议信息
- 时间：
- 参与者：
- 主题：

## 议题与讨论
（按议题列出讨论要点）

## 决议事项
（列出会议决议）

## 待办事项
| 事项 | 负责人 | 截止日期 |
|------|--------|----------|
|      |        |          |

## 下次会议
（时间和议题）

---
> 生成时间：{时间} | 字数：{字数}`

	case TemplateTechReview:
		return `请按以下结构生成技术评测：
# {标题} - 技术评测 ({日期})

## 评测概述
（技术背景和评测目标）

## 技术原理
（核心技术原理简述）

## 优势分析
（技术优势和亮点）

## 劣势与局限
（不足之处和使用限制）

## 适用场景
（推荐的使用场景）

## 对比分析
（与同类技术的对比）

## 评估结论
（综合评价和建议）

---
> 生成时间：{时间} | 字数：{字数}`

	default: // general
		return `请按以下结构生成通用文档：
# {标题} ({日期})

## 概述
（内容概要）

## 主要内容
（按逻辑组织内容，使用合适的标题层级）

## 要点总结
（关键要点归纳）

## 参考资料
（如有引用来源则列出）

---
> 生成时间：{时间} | 字数：{字数}`
	}
}

// saveDocument 保存文档到文件
func (fa *FileGenerationAgent) saveDocument(title, content string) string {
	if fa.outputDir == "" {
		return ""
	}

	os.MkdirAll(fa.outputDir, 0755)

	dateStr := time.Now().Format("2006-01-02")
	safeTitle := sanitizeFileName(title)
	if len(safeTitle) > 30 {
		safeTitle = safeTitle[:30]
	}
	fileName := fmt.Sprintf("%s_%s.md", dateStr, safeTitle)
	filePath := filepath.Join(fa.outputDir, fileName)

	// 如果文件已存在，添加序号
	counter := 1
	for {
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			break
		}
		fileName = fmt.Sprintf("%s_%s_%d.md", dateStr, safeTitle, counter)
		filePath = filepath.Join(fa.outputDir, fileName)
		counter++
	}

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return ""
	}

	return filePath
}

// sanitizeFileName 清理文件名中的非法字符
func sanitizeFileName(name string) string {
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	name = strings.ReplaceAll(name, " ", "_")
	return name
}
