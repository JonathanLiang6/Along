package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/models"
	"ai-companion/internal/pipeline"
)

// WorkflowStepReporter 步骤级进度回调；status 取值 running/success/failed。
// 用于步骤执行记录持久化与前端画布运行高亮；可为 nil。
type WorkflowStepReporter func(stepIndex int, stepName, status, outputPreview string)

// RunWorkflow 执行 workflow 任务的图结构步骤序列（Agent 协作链）。
// 节点（AutomationStep）以 step_index 唯一标识；出边由 next_on_success /
// next_on_failure 表示：>0 跳转到对应节点、0 走排序后的下一节点（兼容旧线性数据）、
// -1 结束、-2 重试本步（旧数据兼容，上限 3 次）。
// 支持条件分支（condition / repeat）、回跳形成循环、以及多入边合并汇入。
// 变量经 vars 在节点间传递，file_path 等路径支持 {{date}} 等变量。
func (cc *CompanionCore) RunWorkflow(task *models.AutomationTask, steps []models.AutomationStep, reporter WorkflowStepReporter) models.TaskResult {
	vars := make(map[string]interface{})
	var lastContent string
	var lastFilePath string
	var notifications []string
	success := true
	statusText := ""
	stepDone := 0
	retryCounts := make(map[int]int)
	iterations := 0

	if len(steps) == 0 {
		return models.TaskResult{Success: false, StatusText: "流程为空"}
	}

	// 按 step_index 升序排序，构建索引；"0 走排序后的下一节点"依赖此顺序
	sorted := make([]models.AutomationStep, len(steps))
	copy(sorted, steps)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].StepIndex < sorted[j].StepIndex })
	byIndex := make(map[int]models.AutomationStep, len(sorted))
	for _, s := range sorted {
		byIndex[s.StepIndex] = s
	}

	// nextOf 解析节点出边：takeSuccess 决定走 success 还是 failure 边
	nextOf := func(curIdx int, takeSuccess bool) int {
		step := byIndex[curIdx]
		target := step.NextOnSuccess
		if !takeSuccess {
			target = step.NextOnFailure
		}
		if target == -2 {
			return -2 // 重试本步
		}
		if target > 0 {
			if _, ok := byIndex[target]; ok {
				return target
			}
			return -1 // 目标节点不存在，结束
		}
		if target == 0 {
			for i := range sorted {
				if sorted[i].StepIndex == curIdx {
					if i+1 < len(sorted) {
						return sorted[i+1].StepIndex
					}
					break
				}
			}
			return -1
		}
		return -1
	}

	visitCount := make(map[int]int)
	maxVisitsPerNode := 500
	maxIterations := len(sorted)*500 + 50

	curIdx := sorted[0].StepIndex
	for curIdx >= 0 {
		step := byIndex[curIdx]
		cfg := parseStepConfig(step.Config)

		// 循环防护：单节点访问上限 + 全局执行上限
		iterations++
		visitCount[curIdx]++
		if visitCount[curIdx] > maxVisitsPerNode {
			success = false
			statusText = fmt.Sprintf("节点[%s]访问次数过多，已中止（可能存在死循环）", step.Name)
			break
		}
		if iterations > maxIterations {
			success = false
			statusText = "流程执行次数过多，已中止（可能存在循环跳转）"
			break
		}

		// start 节点：纯入口标记，沿 success 边继续
		if step.StepType == "start" {
			curIdx = nextOf(curIdx, true)
			continue
		}

		if reporter != nil {
			reporter(step.StepIndex, step.Name, "running", "")
		}

		var stepOut string
		var stepFile string
		var stepErr error
		takeSuccess := true

		switch step.StepType {
		case "web_search", "search":
			if engine := getConfigString(cfg, "engine", ""); engine != "" && cc.webAgent != nil {
				cc.webAgent.SetSearchProvider(engine)
			}
			query := replaceVarsText(getConfigString(cfg, "query", ""), vars)
			if strings.TrimSpace(query) == "" {
				query = task.Name
			}
			stepOut, stepErr = cc.runWebSearch(query, getConfigInt(cfg, "result_count", 10))
			// 可选：AI 总结搜索结果（search 步骤的 need_summary 开关）
			if stepErr == nil && getConfigBool(cfg, "need_summary", false) && strings.TrimSpace(stepOut) != "" {
				if summary, err := cc.runSummarize(query, stepOut, getConfigString(cfg, "summary_type", "detailed")); err == nil {
					stepOut = summary
				}
			}
		case "summarize":
			raw := cc.pickRawContent(cfg, vars, lastContent)
			stepOut, stepErr = cc.runSummarize(
				getConfigString(cfg, "topic", task.Name),
				raw,
				getConfigString(cfg, "summary_type", "detailed"),
			)
		case "agent_chat", "agent":
			prompt := replaceVarsText(getConfigString(cfg, "prompt", step.Name), vars)
			if agentName := getConfigString(cfg, "agent_name", ""); agentName != "" {
				stepOut, stepErr = cc.runNamedAgent(agentName, prompt)
			} else {
				stepOut, stepErr = cc.runAgentChat(prompt)
			}
		case "file_generation", "save_file":
			raw := cc.pickRawContent(cfg, vars, lastContent)
			title := getConfigString(cfg, "title", task.Name)
			filePath := resolveSavePath(cfg, vars)
			format := getConfigString(cfg, "format", "")
			// 纯文本 / JSON：不经过 AI，直接写入文件
			if format == "text" || format == "json" {
				saved := cc.writeRawFile(filePath, raw)
				if saved == "" && filePath != "" {
					stepErr = fmt.Errorf("写入文件失败: %s", filePath)
				} else {
					stepOut = raw
					stepFile = saved
				}
			} else {
				stepOut, stepFile, stepErr = cc.runFileGeneration(raw, title, getConfigString(cfg, "template", "general"), filePath)
			}
		case "condition":
			expr := buildConditionExpr(cfg, vars)
			if pipeline.EvaluateCondition(expr, toStringVars(vars)) {
				stepOut = "条件满足，走成功分支"
				takeSuccess = true
			} else {
				stepOut = "条件不满足，走失败分支"
				takeSuccess = false
			}
		case "repeat":
			// 循环计数保存在 vars 中：未达上限走 success 边（进入循环体），
			// 达到上限后走 failure 边（退出循环）
			countKey := fmt.Sprintf("__repeat_%d", step.StepIndex)
			maxIters := getConfigInt(cfg, "max_iterations", 1)
			if maxIters < 1 {
				maxIters = 1
			}
			n := 0
			if v, ok := vars[countKey].(int); ok {
				n = v
			}
			if n < maxIters {
				vars[countKey] = n + 1
				stepOut = fmt.Sprintf("第 %d 次迭代", n+1)
				takeSuccess = true
			} else {
				delete(vars, countKey)
				stepOut = "迭代完成，退出循环"
				takeSuccess = false
			}
		case "set_variable":
			name := getConfigString(cfg, "name", "")
			value := replaceVarsText(getConfigString(cfg, "value", ""), vars)
			if name == "" {
				stepErr = fmt.Errorf("set_variable 未配置变量名")
			} else {
				vars[name] = value
				stepOut = value
			}
		case "delay":
			secs := getConfigInt(cfg, "seconds", 0)
			if secs > 3600 {
				secs = 3600
			}
			if secs > 0 {
				time.Sleep(time.Duration(secs) * time.Second)
			}
			stepOut = fmt.Sprintf("已等待 %d 秒", secs)
		case "web_fetch":
			url := replaceVarsText(getConfigString(cfg, "url", ""), vars)
			if strings.TrimSpace(url) == "" {
				stepErr = fmt.Errorf("web_fetch 未配置URL")
			} else {
				stepOut, stepErr = cc.runWebFetch(url)
			}
		case "research":
			query := replaceVarsText(getConfigString(cfg, "query", task.Name), vars)
			stepOut, stepErr = cc.runResearch(query)
		case "reflection":
			period := getConfigString(cfg, "period", "week")
			stepOut, stepErr = cc.runReflection(period)
		case "memory_recall":
			query := replaceVarsText(getConfigString(cfg, "query", ""), vars)
			stepOut, stepErr = cc.runMemoryRecall(query)
		case "tech_analysis":
			topic := replaceVarsText(getConfigString(cfg, "topic", task.Name), vars)
			stepOut, stepErr = cc.runTechAnalysis(topic)
		case "notify":
			content := replaceVarsText(
				getConfigString(cfg, "content", getConfigString(cfg, "message", step.Name)),
				vars,
			)
			switch getConfigString(cfg, "level", "normal") {
			case "warning", "warn":
				stepOut = "⚠️ [提醒] " + content
			case "critical", "error", "important":
				stepOut = "🔴 [重要] " + content
			default:
				stepOut = "🔔 " + content
			}
			// 收集通知，由上层统一推送给用户（toast / 系统通知）
			notifications = append(notifications, content)
		default:
			stepErr = fmt.Errorf("不支持的步骤类型: %s", step.StepType)
		}

		if stepErr != nil {
			if reporter != nil {
				reporter(step.StepIndex, step.Name, "failed", stepErr.Error())
			}
			// 失败分支：-2 重试（上限 3 次）、>0 跳转、否则中止
			if step.NextOnFailure == -2 {
				retryCounts[curIdx]++
				if retryCounts[curIdx] > 3 {
					success = false
					statusText = fmt.Sprintf("步骤[%s]重试超过3次，流程已中止", step.StepType)
					break
				}
				continue // 重试本步
			}
			if step.NextOnFailure > 0 {
				if _, ok := byIndex[step.NextOnFailure]; ok {
					curIdx = step.NextOnFailure
					continue
				}
			}
			success = false
			statusText = fmt.Sprintf("步骤[%s]失败: %v", step.StepType, stepErr)
			break
		}

		stepDone++
		if reporter != nil {
			reporter(step.StepIndex, step.Name, "success", stepOut)
		}

		if stepOut != "" {
			lastContent = stepOut
		}
		if stepFile != "" {
			lastFilePath = stepFile
		}
		if step.OutputVar != "" {
			vars[step.OutputVar] = stepOut
		}

		// 解析下一条边
		next := nextOf(curIdx, takeSuccess)
		if next == -2 {
			// 旧数据兼容：条件不满足时重试本步
			retryCounts[curIdx]++
			if retryCounts[curIdx] > 3 {
				success = false
				statusText = fmt.Sprintf("步骤[%s]重试超过3次，流程已中止", step.StepType)
				break
			}
			stepDone--
			if step.OutputVar != "" {
				delete(vars, step.OutputVar)
			}
			continue // 重新执行本步
		}
		if next < 0 {
			break // 结束流程
		}
		curIdx = next
	}

	if success && stepDone == 0 && len(notifications) == 0 {
		success = false
		statusText = "任务没有产生任何输出"
	}

	return models.TaskResult{
		Success:       success,
		StatusText:    statusText,
		ResultType:    "text",
		Content:       lastContent,
		FilePath:      lastFilePath,
		Variables:     vars,
		Duration:      int64(stepDone),
		Notifications: notifications,
	}
}

// RunReminderTask 执行 reminder 任务：读取配置中的提醒文案（message/content），
// 替换 {{date}} 等内置变量后作为结果返回并标记为需推送通知。
// 之前该类型被错误地丢给 LLM Orchestrator，导致文案丢失且常报"无法调用工具"。
func (cc *CompanionCore) RunReminderTask(task *models.AutomationTask) models.TaskResult {
	cfg := parseStepConfig(task.Config)
	msg := getConfigString(cfg, "message", "")
	if msg == "" {
		msg = getConfigString(cfg, "content", "")
	}
	if msg == "" {
		msg = task.Description
	}
	if strings.TrimSpace(msg) == "" {
		return models.TaskResult{
			Success:    false,
			StatusText: "提醒内容为空",
			ResultType: "text",
		}
	}
	msg = replaceVarsText(msg, varsForTask(task))
	return models.TaskResult{
		Success:       true,
		StatusText:    "success",
		ResultType:    "text",
		Content:       msg,
		Notifications: []string{msg},
	}
}

// RunReflectionTask 执行 reflection 类型任务：调用反思 Agent 按周期生成复盘。
// 之前这类任务被丢给 LLM Orchestrator 兜底，结果不稳定；现在走专用分支。
func (cc *CompanionCore) RunReflectionTask(task *models.AutomationTask) models.TaskResult {
	cfg := parseStepConfig(task.Config)
	period := getConfigString(cfg, "period", "week")
	content, err := cc.runReflection(period)
	if err != nil {
		return models.TaskResult{Success: false, StatusText: err.Error(), ResultType: "text"}
	}
	result := models.TaskResult{
		Success:    true,
		StatusText: "success",
		ResultType: "text",
		Content:    content,
	}
	// 可选保存为文件
	if getConfigString(cfg, "output_type", "file") == "file" {
		filePath := replaceVarsText(getConfigString(cfg, "file_path", ""), varsForTask(task))
		doc, path, ferr := cc.runFileGeneration(content, task.Name, "weekly", filePath)
		if ferr == nil && path != "" {
			result.Content = doc
			result.FilePath = path
		}
	}
	return result
}

// RunReportTask 执行 report 类型任务：汇总近期对话与任务执行情况，生成日报/周报/月报。
func (cc *CompanionCore) RunReportTask(task *models.AutomationTask) models.TaskResult {
	cfg := parseStepConfig(task.Config)
	period := getConfigString(cfg, "period", "weekly")
	periodLabel := map[string]string{"daily": "日", "weekly": "周", "monthly": "月"}[period]
	if periodLabel == "" {
		periodLabel = "周"
	}

	// 1. 汇总近期用户对话
	var recentConvs []string
	if cc.conversationService != nil {
		if msgs, err := cc.conversationService.GetRecentMessages(120); err == nil {
			for _, m := range msgs {
				if strings.TrimSpace(m.Content) != "" {
					recentConvs = append(recentConvs, m.Content)
				}
			}
		}
	}
	// 2. 汇总近期执行的任务
	var recentTasks []string
	if cc.automationService != nil {
		if tasks, err := cc.automationService.GetTasks(""); err == nil {
			for _, t := range tasks {
				if t.LastRunAt != "" && !strings.HasPrefix(t.LastRunAt, "0001") {
					recentTasks = append(recentTasks, t.Name)
				}
			}
		}
	}

	var parts []string
	if len(recentConvs) > 0 {
		parts = append(parts, "近期对话摘录（按时间）：\n"+strings.Join(recentConvs, "\n"))
	}
	if len(recentTasks) > 0 {
		parts = append(parts, "近期执行的任务："+strings.Join(recentTasks, "、"))
	}
	raw := "请根据以下素材生成本" + periodLabel + "报告（涵盖成果、问题与下周建议）：\n" + strings.Join(parts, "\n")

	// 3. 交给总结 Agent 生成结构化报告；失败时降级为反思 Agent
	content, err := cc.runSummarize("本"+periodLabel+"报告", raw, "detailed")
	if err != nil {
		if fallback, rerr := cc.runReflection(period); rerr == nil {
			content = fallback
		} else {
			return models.TaskResult{Success: false, StatusText: rerr.Error(), ResultType: "text"}
		}
	}

	result := models.TaskResult{
		Success:    true,
		StatusText: "success",
		ResultType: "text",
		Content:    content,
	}
	if getConfigString(cfg, "output_type", "file") == "file" {
		filePath := replaceVarsText(getConfigString(cfg, "file_path", ""), varsForTask(task))
		doc, path, ferr := cc.runFileGeneration(content, task.Name, "weekly", filePath)
		if ferr == nil && path != "" {
			result.Content = doc
			result.FilePath = path
		}
	}
	return result
}

// varsForTask 构造任务级内置变量（{{date}}/{{time}}），供文件路径替换使用
func varsForTask(task *models.AutomationTask) map[string]interface{} {
	return map[string]interface{}{
		"date": time.Now().Format("2006-01-02"),
		"time": time.Now().Format("15:04:05"),
	}
}

// RunWebSearchTask 执行 web_search 类型任务（默认 /research 任务）
// 流程：搜索 → （可选）AI总结 → （可选）保存为文件
// 搜索失败时不中断，降级为基于已有知识生成，保证报告始终产出
func (cc *CompanionCore) RunWebSearchTask(task *models.AutomationTask) models.TaskResult {
	cfg := parseStepConfig(task.Config)
	query := getConfigString(cfg, "query", "")
	if strings.TrimSpace(query) == "" {
		query = task.Name
	}
	resultCount := getConfigInt(cfg, "result_count", 10)
	needSummary := getConfigBool(cfg, "need_summary", true)
	outputType := getConfigString(cfg, "output_type", "text")
	filePath := replaceVarsText(getConfigString(cfg, "file_path", ""), map[string]interface{}{})

	// 使用配置指定的搜索源（未配置则用默认可达源）
	if engine := getConfigString(cfg, "engine", ""); engine != "" && cc.webAgent != nil {
		cc.webAgent.SetSearchProvider(engine)
	}

	// 1. 搜索（失败/无结果时降级，不中断报告生成）
	searchFailed := false
	formatted := ""
	if cc.webAgent == nil {
		searchFailed = true
		formatted = "（联网搜索不可用，以下为基于已有知识的生成内容）"
	} else {
		results, err := cc.webAgent.Search(query)
		if err != nil {
			searchFailed = true
			formatted = fmt.Sprintf("（联网搜索失败：%v\n以下为基于已有知识的生成内容）", err)
		} else {
			formatted = formatSearchResults(results, resultCount)
			if formatted == "" {
				searchFailed = true
				formatted = "（未获取到搜索结果，以下为基于已有知识的生成内容）"
			}
		}
	}

	// 2. 总结
	if needSummary {
		summary, serr := cc.runSummarize(query, formatted, "detailed")
		if serr == nil && strings.TrimSpace(summary) != "" {
			formatted = summary
		}
	}

	// 3. 保存文件
	var savedPath string
	if outputType == "file" {
		// 把搜索/总结结果作为原始内容交给 file_generation 重新格式化成研究文档
		doc, path, ferr := cc.runFileGeneration(formatted, task.Name, "research", filePath)
		if ferr == nil && path != "" {
			formatted = doc
			savedPath = path
		}
	}

	statusText := "success"
	if searchFailed {
		statusText = "completed_with_search_fallback"
	}

	return models.TaskResult{
		Success:    true,
		StatusText: statusText,
		ResultType: "text",
		Content:    formatted,
		FilePath:   savedPath,
	}
}

// ==================== 步骤执行原语 ====================

// runWebSearch 执行搜索，返回格式化后的搜索结果文本
func (cc *CompanionCore) runWebSearch(query string, resultCount int) (string, error) {
	if cc.webAgent == nil {
		return "", fmt.Errorf("web agent 未初始化")
	}
	results, err := cc.webAgent.Search(query)
	if err != nil {
		return "", err
	}
	return formatSearchResults(results, resultCount), nil
}

// runSummarize 执行信息整合
func (cc *CompanionCore) runSummarize(topic, rawContent, summaryType string) (string, error) {
	if cc.summarizeAgent == nil {
		return "", fmt.Errorf("summarize agent 未初始化")
	}
	ctx := agents.AgentContext{
		Content: topic,
		Extra: map[string]interface{}{
			"query":        topic,
			"raw_content":  rawContent,
			"summary_type": summaryType,
		},
	}
	result, err := cc.summarizeAgent.Process(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runAgentChat 通过 Orchestrator 执行自然语言任务（自动规划 + 上下文注入）
func (cc *CompanionCore) runAgentChat(prompt string) (string, error) {
	if cc.orchestrator == nil {
		return "", fmt.Errorf("orchestrator 未初始化")
	}
	result, err := cc.orchestrator.Process(prompt)
	if err != nil {
		return "", err
	}
	if result == nil {
		return "", fmt.Errorf("orchestrator 返回空结果")
	}
	return result.Content, nil
}

// runNamedAgent 直接调用指定名称的 Agent
func (cc *CompanionCore) runNamedAgent(name, prompt string) (string, error) {
	agent, ok := cc.agentManager.GetAgent(name)
	if !ok {
		return "", fmt.Errorf("找不到 Agent: %s", name)
	}
	ctx := agents.AgentContext{Content: prompt}
	result, err := agent.Process(ctx)
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runResearch 深度调研（多角度搜索 + 交叉验证）
func (cc *CompanionCore) runResearch(query string) (string, error) {
	if cc.researchAgent == nil {
		return "", fmt.Errorf("research agent 未初始化")
	}
	result, err := cc.researchAgent.Process(agents.AgentContext{Content: query})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runReflection 生成周期复盘（period: day/week/month）
func (cc *CompanionCore) runReflection(period string) (string, error) {
	if cc.reflectionAgent == nil {
		return "", fmt.Errorf("reflection agent 未初始化")
	}
	prompt := "请生成本周期的复盘报告"
	if period != "" {
		prompt = fmt.Sprintf("请生成本周期（%s）的复盘报告", period)
	}
	result, err := cc.reflectionAgent.Process(agents.AgentContext{Content: prompt})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runMemoryRecall 记忆查询
func (cc *CompanionCore) runMemoryRecall(query string) (string, error) {
	if cc.memoryAgent == nil {
		return "", fmt.Errorf("memory agent 未初始化")
	}
	result, err := cc.memoryAgent.Process(agents.AgentContext{Content: query})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runTechAnalysis 技术概念深度分析
func (cc *CompanionCore) runTechAnalysis(topic string) (string, error) {
	if cc.techAnalysisAgent == nil {
		return "", fmt.Errorf("tech_analysis agent 未初始化")
	}
	result, err := cc.techAnalysisAgent.Process(agents.AgentContext{Content: topic})
	if err != nil {
		return "", err
	}
	return result.Content, nil
}

// runWebFetch 抓取网页内容
func (cc *CompanionCore) runWebFetch(url string) (string, error) {
	if cc.webAgent == nil {
		return "", fmt.Errorf("web agent 未初始化")
	}
	content, err := cc.webAgent.FetchPageContent(url)
	if err != nil {
		return "", err
	}
	return content, nil
}

// runFileGeneration 生成并保存文档，返回（文档内容, 保存路径, 错误）
func (cc *CompanionCore) runFileGeneration(rawContent, title, template, filePath string) (string, string, error) {
	if cc.fileGenerationAgent == nil {
		return "", "", fmt.Errorf("file_generation agent 未初始化")
	}
	ctx := agents.AgentContext{
		Content: rawContent,
		Extra: map[string]interface{}{
			"raw_content": rawContent,
			"title":       title,
			"template":    template,
		},
	}
	if filePath != "" {
		ctx.Extra["file_path"] = filePath
	}
	result, err := cc.fileGenerationAgent.Process(ctx)
	if err != nil {
		return "", "", err
	}
	path := ""
	if result.Data != nil {
		if dataMap, ok := result.Data.(map[string]interface{}); ok {
			if fp, ok := dataMap["file_path"].(string); ok {
				path = fp
			}
		}
	}
	return result.Content, path, nil
}

// buildConditionExpr 由前端条件配置（source_var/operator/compare_value）构造求值表达式
func buildConditionExpr(cfg map[string]interface{}, vars map[string]interface{}) string {
	if expr := getConfigString(cfg, "expression", ""); expr != "" {
		return replaceVarsText(expr, vars)
	}
	if cond := getConfigString(cfg, "condition", ""); cond != "" {
		return replaceVarsText(cond, vars)
	}
	sourceVar := getConfigString(cfg, "source_var", "")
	operator := getConfigString(cfg, "operator", "contains")
	compare := strings.ReplaceAll(getConfigString(cfg, "compare_value", ""), `"`, `\"`)
	if sourceVar == "" {
		return "true"
	}
	switch operator {
	case "not_contains":
		return fmt.Sprintf("{{%s}} not_contains %q", sourceVar, compare)
	case "equals":
		return fmt.Sprintf("{{%s}} == %q", sourceVar, compare)
	case "is_empty":
		return fmt.Sprintf("{{%s}} is_empty", sourceVar)
	case "not_empty":
		return fmt.Sprintf("{{%s}} not_empty", sourceVar)
	default: // contains
		return fmt.Sprintf("{{%s}} contains %q", sourceVar, compare)
	}
}

// resolveSavePath 解析 save_file 步骤的保存路径：
// file_name + file_path（作为目录）组合，或 file_path 直接作为完整路径（模板风格）
func resolveSavePath(cfg map[string]interface{}, vars map[string]interface{}) string {
	dir := replaceVarsText(getConfigString(cfg, "file_path", ""), vars)
	fileName := replaceVarsText(getConfigString(cfg, "file_name", ""), vars)
	if fileName == "" {
		return dir
	}
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, fileName)
}

// writeRawFile 直接写入文本/JSON 文件，返回保存路径；失败返回空字符串
func (cc *CompanionCore) writeRawFile(path, content string) string {
	if path == "" {
		return ""
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return ""
	}
	// 已存在则追加序号
	if _, err := os.Stat(path); err == nil {
		ext := filepath.Ext(path)
		base := strings.TrimSuffix(path, ext)
		counter := 1
		for {
			candidate := fmt.Sprintf("%s_%d%s", base, counter, ext)
			if _, err := os.Stat(candidate); os.IsNotExist(err) {
				path = candidate
				break
			}
			counter++
		}
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return ""
	}
	return path
}

// pickRawContent 按优先级选取 file_generation/summarize 步骤的原始内容：
// content_var > use_raw_from > raw_content 配置 > 上一步输出
func (cc *CompanionCore) pickRawContent(cfg map[string]interface{}, vars map[string]interface{}, lastContent string) string {
	if key := getConfigString(cfg, "content_var", ""); key != "" {
		if v, ok := vars[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	if key := getConfigString(cfg, "use_raw_from", ""); key != "" {
		if v, ok := vars[key]; ok {
			return fmt.Sprintf("%v", v)
		}
	}
	if raw := getConfigString(cfg, "raw_content", ""); raw != "" {
		return replaceVarsText(raw, vars)
	}
	return lastContent
}

// ==================== 配置与变量工具 ====================

func parseStepConfig(configJSON string) map[string]interface{} {
	var cfg map[string]interface{}
	if configJSON != "" {
		json.Unmarshal([]byte(configJSON), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]interface{})
	}
	return cfg
}

func getConfigString(cfg map[string]interface{}, key, def string) string {
	if v, ok := cfg[key]; ok {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return fmt.Sprintf("%.0f", t)
		default:
			return fmt.Sprintf("%v", t)
		}
	}
	return def
}

func getConfigInt(cfg map[string]interface{}, key string, def int) int {
	if v, ok := cfg[key]; ok {
		switch t := v.(type) {
		case float64:
			return int(t)
		case int:
			return t
		case string:
			var n int
			fmt.Sscanf(t, "%d", &n)
			return n
		}
	}
	return def
}

func getConfigBool(cfg map[string]interface{}, key string, def bool) bool {
	if v, ok := cfg[key]; ok {
		switch t := v.(type) {
		case bool:
			return t
		case string:
			return t == "true" || t == "1"
		case float64:
			return t != 0
		}
	}
	return def
}

// replaceVarsText 用变量表替换字符串中的 {{key}}
func replaceVarsText(s string, vars map[string]interface{}) string {
	if vars == nil {
		vars = make(map[string]interface{})
	}
	// 内置变量（日期/时间）
	now := time.Now()
	vars["date"] = now.Format("2006-01-02")
	vars["time"] = now.Format("15:04:05")
	vars["datetime"] = now.Format("2006-01-02 15:04:05")
	vars["year"] = now.Format("2006")
	vars["month"] = now.Format("01")
	vars["day"] = now.Format("02")

	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", fmt.Sprintf("%v", v))
	}
	return s
}

// toStringVars 将 map[string]interface{} 转为 map[string]string（供条件求值）
func toStringVars(vars map[string]interface{}) map[string]string {
	out := make(map[string]string, len(vars))
	for k, v := range vars {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// formatSearchResults 将搜索结果格式化为文本
func formatSearchResults(results []agents.SearchResult, limit int) string {
	if limit <= 0 {
		limit = 10
	}
	var sb strings.Builder
	count := 0
	for i, r := range results {
		if i >= limit {
			break
		}
		count++
		sb.WriteString(fmt.Sprintf("%d. %s\n", count, r.Title))
		if r.Snippet != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Snippet))
		}
		if r.Link != "" {
			sb.WriteString(fmt.Sprintf("   来源: %s\n", r.Link))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}
