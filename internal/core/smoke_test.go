package core

import (
	"encoding/json"
	"strings"
	"testing"

	"ai-companion/internal/models"
)

func cfg(m map[string]interface{}) string {
	b, _ := json.Marshal(m)
	return string(b)
}

func step(idx int, typ, name, config string, nextSucc, nextFail int) models.AutomationStep {
	return models.AutomationStep{
		StepIndex:     idx,
		StepType:      typ,
		Name:          name,
		Config:        config,
		NextOnSuccess: nextSucc,
		NextOnFailure: nextFail,
	}
}

func TestWorkflowBranch(t *testing.T) {
	steps := []models.AutomationStep{
		step(0, "start", "开始", "", 1, -1),
		step(1, "set_variable", "设置城市", cfg(map[string]interface{}{"name": "city", "value": "北京"}), 2, -1),
		step(2, "condition", "判断", cfg(map[string]interface{}{"expression": "{{city}} == 北京"}), 3, 4),
		step(3, "notify", "成功分支", cfg(map[string]interface{}{"content": "所在城市：{{city}}"}), -1, -1),
		step(4, "notify", "失败分支", cfg(map[string]interface{}{"content": "条件不满足分支"}), -1, -1),
	}
	cc := &CompanionCore{}
	res := cc.RunWorkflow(&models.AutomationTask{Name: "test", Config: "{}"}, steps, nil)
	if !res.Success {
		t.Fatalf("期望成功，得到失败: %s", res.StatusText)
	}
	if len(res.Notifications) != 1 || res.Notifications[0] != "所在城市：北京" {
		t.Fatalf("期望走成功分支并替换变量，得到: %v", res.Notifications)
	}
}

func TestWorkflowRepeatLoop(t *testing.T) {
	// repeat(3) → notify → 回跳 repeat；达到上限走 failure 边结束
	steps := []models.AutomationStep{
		step(0, "start", "开始", "", 1, -1),
		step(1, "repeat", "循环", cfg(map[string]interface{}{"max_iterations": 3}), 2, -1),
		step(2, "notify", "迭代输出", cfg(map[string]interface{}{"content": "迭代{{__repeat_1}}"}), 1, -1),
	}
	cc := &CompanionCore{}
	res := cc.RunWorkflow(&models.AutomationTask{Name: "test", Config: "{}"}, steps, nil)
	if !res.Success {
		t.Fatalf("期望成功，得到失败: %s", res.StatusText)
	}
	if len(res.Notifications) != 3 {
		t.Fatalf("期望 3 次迭代，得到 %d 条通知: %v", len(res.Notifications), res.Notifications)
	}
}

func TestWorkflowLoopProtection(t *testing.T) {
	// 自环且永不满足退出条件 → 应被循环防护中止，而不是无限执行
	steps := []models.AutomationStep{
		step(0, "start", "开始", "", 1, -1),
		step(1, "notify", "自环", cfg(map[string]interface{}{"content": "x"}), 1, -1),
	}
	cc := &CompanionCore{}
	res := cc.RunWorkflow(&models.AutomationTask{Name: "test", Config: "{}"}, steps, nil)
	if res.Success {
		t.Fatal("期望循环被检测并中止，却返回成功")
	}
	if !strings.Contains(res.StatusText, "死循环") && !strings.Contains(res.StatusText, "中止") {
		t.Fatalf("期望中止原因含循环提示，得到: %s", res.StatusText)
	}
}

func TestWorkflowEmpty(t *testing.T) {
	cc := &CompanionCore{}
	res := cc.RunWorkflow(&models.AutomationTask{Name: "test", Config: "{}"}, nil, nil)
	if res.Success {
		t.Fatal("空流程应失败")
	}
}
