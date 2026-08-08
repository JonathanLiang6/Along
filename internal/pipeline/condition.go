package pipeline

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// EvaluateCondition 增强条件表达式求值
// 支持: ==, !=, >, <, >=, <=, contains, not_contains, is_empty, not_empty
// 支持: && (与), || (或), ! (非), () 括号分组
func EvaluateCondition(expr string, vars map[string]string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" || expr == "true" {
		return true
	}
	if expr == "false" {
		return false
	}

	// 替换 {{var}} 为实际值
	resolved := resolveVars(expr, vars)

	// 按 || 拆分（最低优先级）
	orParts := splitBy(resolved, "||")
	for _, part := range orParts {
		if evalAnd(strings.TrimSpace(part)) {
			return true
		}
	}
	return false
}

// evalAnd 处理 && 连接的条件
func evalAnd(expr string) bool {
	parts := splitBy(expr, "&&")
	for _, part := range parts {
		if !evalNot(strings.TrimSpace(part)) {
			return false
		}
	}
	return true
}

// evalNot 处理 ! 取反
func evalNot(expr string) bool {
	if strings.HasPrefix(expr, "!") {
		return !evalParen(strings.TrimSpace(expr[1:]))
	}
	return evalParen(expr)
}

// evalParen 处理括号分组
func evalParen(expr string) bool {
	expr = strings.TrimSpace(expr)
	for strings.HasPrefix(expr, "(") && strings.HasSuffix(expr, ")") {
		// 确保括号是匹配的
		if isMatchingParen(expr) {
			expr = strings.TrimSpace(expr[1 : len(expr)-1])
		} else {
			break
		}
	}
	return evalSingle(expr)
}

// isMatchingParen 检查首尾括号是否成对
func isMatchingParen(s string) bool {
	if !strings.HasPrefix(s, "(") || !strings.HasSuffix(s, ")") {
		return false
	}
	depth := 0
	runes := []rune(s)
	for i, r := range runes {
		if r == '(' {
			depth++
		}
		if r == ')' {
			depth--
		}
		if depth == 0 && i < len(runes)-1 {
			return false
		}
	}
	return depth == 0
}

// splitBy 按分隔符拆分（忽略括号内的分隔符）
func splitBy(s, sep string) []string {
	var parts []string
	depth := 0
	start := 0

	for i := 0; i < len(s); {
		// 检查括号
		if s[i] == '(' {
			depth++
			i++
			continue
		}
		if s[i] == ')' {
			depth--
			i++
			continue
		}
		// 在括号外检查分隔符
		if depth == 0 && i+len(sep) <= len(s) && s[i:i+len(sep)] == sep {
			parts = append(parts, strings.TrimSpace(s[start:i]))
			i += len(sep)
			start = i
			continue
		}
		i++
	}

	if start < len(s) {
		parts = append(parts, strings.TrimSpace(s[start:]))
	}
	return parts
}

// resolveVars 替换字符串中的 {{key}} 变量（含实时时间变量）
func resolveVars(s string, vars map[string]string) string {
	// 先替换用户自定义变量
	for k, v := range vars {
		s = strings.ReplaceAll(s, fmt.Sprintf("{{%s}}", k), v)
	}

	// 再替换实时时间变量（精确到当前时刻）
	now := time.Now()
	timeVars := map[string]string{
		"date":     now.Format("2006-01-02"),
		"time":     now.Format("15:04:05"),
		"datetime": now.Format("2006-01-02 15:04:05"),
		"year":     now.Format("2006"),
		"month":    now.Format("01"),
		"day":      now.Format("02"),
		"hour":     now.Format("15"),
		"minute":   now.Format("04"),
		"second":   now.Format("05"),
		"weekday":  chineseWeekday(now.Weekday()),
		"timestamp": fmt.Sprintf("%d", now.Unix()),
	}
	for k, v := range timeVars {
		s = strings.ReplaceAll(s, fmt.Sprintf("{{%s}}", k), v)
	}

	return s
}

// chineseWeekday 返回中文星期
func chineseWeekday(w time.Weekday) string {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	return names[int(w)]
}

// evalSingle 求值单个条件表达式
func evalSingle(cond string) bool {
	cond = strings.TrimSpace(cond)

	if cond == "" || cond == "true" {
		return true
	}
	if cond == "false" {
		return false
	}

	// is_empty / not_empty
	if strings.HasSuffix(cond, " is_empty") {
		val := strings.Trim(strings.TrimSpace(strings.TrimSuffix(cond, " is_empty")), "\"'")
		return val == "" || val == "null" || val == "nil"
	}
	if strings.HasSuffix(cond, " not_empty") {
		val := strings.Trim(strings.TrimSpace(strings.TrimSuffix(cond, " not_empty")), "\"'")
		return val != "" && val != "null" && val != "nil"
	}

	// contains / not_contains
	for _, op := range []string{" contains ", " not_contains "} {
		if idx := strings.Index(cond, op); idx >= 0 {
			left := strings.TrimSpace(cond[:idx])
			right := strings.Trim(strings.TrimSpace(cond[idx+len(op):]), "\"'")
			if op == " contains " {
				return strings.Contains(left, right)
			}
			return !strings.Contains(left, right)
		}
	}

	// 比较运算符: >=, <=, !=, ==, >, <
	ops := []string{">=", "<=", "!=", "==", ">", "<"}
	for _, op := range ops {
		if idx := strings.Index(cond, " "+op+" "); idx >= 0 {
			left := strings.Trim(strings.TrimSpace(cond[:idx]), "\"'")
			right := strings.Trim(strings.TrimSpace(cond[idx+len(" "+op+" "):]), "\"'")
			return compare(left, op, right)
		}
	}

	// 默认判断
	return cond != "" && cond != "null" && cond != "nil"
}

// compare 比较两个字符串（优先数值比较）
func compare(a, op, b string) bool {
	// 尝试数值比较
	if af, e1 := strconv.ParseFloat(a, 64); e1 == nil {
		if bf, e2 := strconv.ParseFloat(b, 64); e2 == nil {
			switch op {
			case "==":
				return af == bf
			case "!=":
				return af != bf
			case ">":
				return af > bf
			case "<":
				return af < bf
			case ">=":
				return af >= bf
			case "<=":
				return af <= bf
			}
		}
	}
	// 字符串比较
	switch op {
	case "==":
		return a == b
	case "!=":
		return a != b
	case ">":
		return a > b
	case "<":
		return a < b
	case ">=":
		return a >= b
	case "<=":
		return a <= b
	}
	return false
}
