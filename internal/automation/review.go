package automation

import (
	"database/sql"
	"fmt"
	"time"

	"ai-companion/internal/agents"
	"ai-companion/internal/ai"
	"ai-companion/internal/models"
	"ai-companion/internal/services"
)

// ReviewExecutor 复习执行器
// 基于艾宾浩斯遗忘曲线，在合适的时间提醒复习记忆
type ReviewExecutor struct {
	agentMgr *agents.AgentManager
	db       *sql.DB
}

// 艾宾浩斯复习间隔（天）
var ebbinghausIntervals = []int{1, 2, 4, 7, 15, 30}

func (e *ReviewExecutor) Execute(config map[string]interface{}, ctx TaskContext) (*models.TaskResult, error) {
	startTime := time.Now()

	memType, _ := config["memory_type"].(string)
	if memType == "" {
		memType = ""
	}
	reviewCount := 5
	if rc, ok := config["review_count"].(float64); ok {
		reviewCount = int(rc)
	}

	// 从DB读取记忆
	memService := services.NewMemoryService(e.db)
	memories, err := memService.GetMemories(memType)
	if err != nil || len(memories) == 0 {
		return &models.TaskResult{
			Success:    false,
			StatusText: "没有可复习的记忆",
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 筛选需要复习的记忆（根据updated_at时间和艾宾浩斯曲线）
	now := time.Now()
	var toReview []models.Memory
	for _, m := range memories {
		if len(toReview) >= reviewCount {
			break
		}
		// 解析updated_at时间
		updatedAt, err := time.Parse("2006-01-02 15:04:05", m.UpdatedAt)
		if err != nil {
			updatedAt, err = time.Parse(time.RFC3339, m.UpdatedAt)
			if err != nil {
				continue
			}
		}
		daysSince := int(now.Sub(updatedAt).Hours() / 24)
		// 如果距离上次更新超过1天，加入复习列表
		if daysSince >= 1 {
			toReview = append(toReview, m)
		}
	}

	if len(toReview) == 0 {
		return &models.TaskResult{
			Success:    true,
			StatusText: "当前没有需要复习的记忆",
			ResultType: "text",
			Content:    "所有记忆都在复习周期内，暂不需要复习。",
			Duration:   time.Since(startTime).Milliseconds(),
		}, nil
	}

	// 构建复习内容
	var reviewContent string
	for i, m := range toReview {
		reviewContent += fmt.Sprintf("%d. [%s] %s\n", i+1, m.Type, m.Content)
	}

	// 调用MemoryAgent提取要点
	agent, ok := e.agentMgr.GetAgent("memory")
	var summary string
	if ok {
		agentCtx := agents.AgentContext{
			Content: "请帮我复习以下记忆，提炼关键要点：\n" + reviewContent,
			History: []ai.Message{},
		}
		response, err := agent.Process(agentCtx)
		if err == nil {
			summary = response.Content
		}
	}

	if summary == "" {
		summary = reviewContent
	}

	result := &models.TaskResult{
		Success:    true,
		StatusText: fmt.Sprintf("复习%d条记忆", len(toReview)),
		ResultType: "notify",
		Content:    summary,
		Duration:   time.Since(startTime).Milliseconds(),
		Variables:  map[string]interface{}{"review_content": reviewContent, "review_count": len(toReview)},
	}

	return result, nil
}

func (e *ReviewExecutor) ConfigSchema() []models.ConfigField {
	return []models.ConfigField{
		{Key: "memory_type", Label: "记忆类型", Type: "select", Options: []models.ConfigOption{
			{Value: "", Label: "全部"},
			{Value: "L1", Label: "个人画像"},
			{Value: "L2", Label: "情感关系"},
			{Value: "L3", Label: "关键事件"},
			{Value: "L4", Label: "项目目标"},
			{Value: "L5", Label: "日常喜好"},
		}},
		{Key: "review_count", Label: "复习数量", Type: "number", Default: "5"},
	}
}
