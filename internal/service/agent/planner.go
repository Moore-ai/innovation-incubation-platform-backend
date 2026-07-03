package agent

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

// PlanStep 计划中的单个步骤
type PlanStep struct {
	Index int      `json:"index"`
	Tools []string `json:"tools"`
	Desc  string   `json:"desc"`
}

// Plan 完整执行计划
type Plan struct {
	Steps []PlanStep `json:"steps"`
}

// buildPlanPrompt 生成规划 Prompt
func buildPlanPrompt(tools []agenttools.Tool) string {
	var sb strings.Builder
	sb.WriteString("你是一个顶级的AI规划专家兼创新孵化平台的智能助手。请分析用户的问题，将其分解成由简单步骤组成的行动计划，尽可能高效。\n\n")
	sb.WriteString("可用工具：\n")
	for _, t := range tools {
		fmt.Fprintf(&sb, "- %s：%s\n", t.Name(), t.Description())
	}
	sb.WriteString("\n示例计划：\n\nPlan:\n1. query_policy_follow(), query_enterprise_info() — 并行获取关注列表和企业信息\n2. search_policy(query=\"关键词\") — 根据结果搜索政策\n")
	sb.WriteString("\n注意事项：\n- 务必以\"Plan:\"开头，每行一个步骤\n- 同一行内用逗号分隔表示并行执行（仅用于无依赖的工具）\n- 有依赖关系的工具放在不同行\n- 每行末尾 — 后写简要说明\n")
	return sb.String()
}

var planLineRe = regexp.MustCompile(`^\d+[\.\)、]\s*`)

// parsePlan 从 LLM 回复中解析执行计划，支持多种格式（Plan: 前缀、markdown 列表、中英文标点）。
func parsePlan(text string, registry *agenttools.ToolRegistry) (*Plan, error) {
	text = strings.TrimSpace(text)

	// 去除可能的代码围栏
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	// 定位计划内容：找 "Plan:" 前缀
	idx := -1
	for _, prefix := range []string{"Plan:", "plan:", "PLAN:", "计划：", "计划:"} {
		if i := strings.Index(text, prefix); i >= 0 {
			idx = i
			break
		}
	}

	// 无前缀时：尝试直接找编号行作为计划开始
	if idx < 0 {
		idx = findFirstNumberedLine(text)
		if idx < 0 {
			return nil, fmt.Errorf("plan not found in response")
		}
	}

	remaining := text[idx:]
	lines := strings.Split(remaining, "\n")
	var steps []PlanStep

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// 跳过空行、代码围栏、模板说明
		if line == "" || strings.HasPrefix(line, "```") || strings.HasPrefix(line, "注意") {
			continue
		}

		// 去掉 markdown 列表前缀 "- "、 "* "
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")

		if !planLineRe.MatchString(line) {
			if len(steps) > 0 {
				break
			}
			continue
		}

		// 去掉编号前缀 "1. "、"1) "、"1、"
		line = planLineRe.ReplaceAllString(line, "")

		// 清除整行的 markdown 标记（加粗、斜体）
		line = cleanMarkdownLine(line)

		// 提取描述
		var desc string
		line, desc = splitDesc(line)

		// 分割并行工具名
		parts := strings.FieldsFunc(line, func(r rune) bool {
			return r == ',' || r == '，' || r == '、'
		})
		var tools []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if toolName := extractToolName(p); toolName != "" {
				if _, ok := registry.Get(toolName); ok {
					tools = append(tools, toolName)
				}
			}
		}
		if len(tools) > 0 {
			steps = append(steps, PlanStep{Index: len(steps) + 1, Tools: tools, Desc: desc})
		}
	}

	if len(steps) == 0 {
		slog.Warn("计划解析失败，LLM 原始输出", "text", text[:min(len(text), 300)])
		return nil, fmt.Errorf("no valid steps found in plan")
	}
	return &Plan{Steps: steps}, nil
}

// findFirstNumberedLine 找到文本中第一个编号行，返回其位置。
func findFirstNumberedLine(text string) int {
	for i, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "- ")
		line = strings.TrimPrefix(line, "* ")
		if planLineRe.MatchString(line) {
			return strings.Index(text, line)
		}
		_ = i
	}
	return -1
}

// cleanMarkdownLine 清除整行中成对的 markdown 加粗标记（** 和 __）。
// 注意：不处理单字符的 * 或 _，因为工具名（如 search_policy）包含下划线。
func cleanMarkdownLine(line string) string {
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")
	return strings.TrimSpace(line)
}

// splitDesc 从行尾提取描述文本。
func splitDesc(line string) (toolPart, desc string) {
	for _, sep := range []string{" — ", " -- ", " --- ", "：", ": "} {
		if dIdx := strings.LastIndex(line, sep); dIdx >= 0 {
			return strings.TrimSpace(line[:dIdx]), strings.TrimSpace(line[dIdx+len(sep):])
		}
	}
	return line, ""
}

// buildReplanFormatHint 生成 replan 时的格式提示。
func buildReplanFormatHint(tools []agenttools.Tool) string {
	var sb strings.Builder
	sb.WriteString("请为剩余步骤制定替代计划。可用工具：")
	for _, t := range tools {
		fmt.Fprintf(&sb, " %s", t.Name())
	}
	sb.WriteString("。只输出：\n\nPlan:\n1. tool(param) — 说明\n\n不要其他文字。")
	return sb.String()
}

// extractToolName 从片段中提取括号前的工具名。
func extractToolName(p string) string {
	p = strings.TrimSpace(p)
	// 去掉可能的 ` 围栏
	p = strings.Trim(p, "`")
	// 去掉可能的 ** __ 标记
	p = strings.Trim(p, "*_")
	if pIdx := strings.IndexByte(p, '('); pIdx > 0 {
		toolName := strings.TrimSpace(p[:pIdx])
		// 二次清理：去掉残留的 markdown 标记
		toolName = strings.Trim(toolName, "*_")
		return toolName
	}
	return ""
}
