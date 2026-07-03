package agent

import (
	"fmt"
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
	sb.WriteString("\n请严格按以下格式输出计划：\n\nPlan:\n1. [工具名]([参数]) — 说明\n2. [工具名]([参数]), [工具名]([参数]) — 同一行用逗号分隔表示并行执行\n\n注意事项：\n- 同一行内逗号分隔的工具会被同时执行，仅用于无数据依赖的场景\n- 有依赖关系的工具放在不同行，按依赖顺序排列\n- 不需要的工具不要列\n")
	return sb.String()
}

var planLineRe = regexp.MustCompile(`^\d+\.\s*`)

// parsePlan 从 LLM 回复中解析执行计划，工具名通过 registry 校验
func parsePlan(text string, registry *agenttools.ToolRegistry) (*Plan, error) {
	idx := strings.Index(text, "Plan:")
	if idx < 0 {
		idx = strings.Index(text, "plan:")
	}
	if idx < 0 {
		idx = strings.Index(text, "PLAN:")
	}
	if idx < 0 {
		return nil, fmt.Errorf("plan not found in response")
	}

	remaining := text[idx:]
	lines := strings.Split(remaining, "\n")
	var steps []PlanStep

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !planLineRe.MatchString(line) {
			if len(steps) > 0 {
				if strings.TrimSpace(line) == "" {
					continue // skip blank lines, don't break
				}
				// Non-blank, non-step line after steps started — likely end of plan
				break
			}
			continue
		}
		// 去掉编号前缀 "1. "
		line = planLineRe.ReplaceAllString(line, "")

		// 分割工具名和描述
		var desc string
		if dIdx := strings.Index(line, "—"); dIdx >= 0 { // Unicode em dash (U+2014)
			desc = strings.TrimSpace(line[dIdx+3:])
			line = strings.TrimSpace(line[:dIdx])
		} else if dIdx := strings.Index(line, "---"); dIdx >= 0 {
			desc = strings.TrimSpace(line[dIdx+3:])
			line = strings.TrimSpace(line[:dIdx])
		} else if dIdx := strings.Index(line, "--"); dIdx >= 0 {
			desc = strings.TrimSpace(line[dIdx+2:])
			line = strings.TrimSpace(line[:dIdx])
		}

		// 分割工具名（逗号或顿号）
		parts := strings.FieldsFunc(line, func(r rune) bool { return r == ',' || r == '、' || r == '，' })
		var tools []string
		for _, p := range parts {
			p = strings.TrimSpace(p)
			// 提取括号前的工具名
			if pIdx := strings.IndexByte(p, '('); pIdx > 0 {
				toolName := strings.TrimSpace(p[:pIdx])
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
		return nil, fmt.Errorf("no valid steps found in plan")
	}
	return &Plan{Steps: steps}, nil
}
