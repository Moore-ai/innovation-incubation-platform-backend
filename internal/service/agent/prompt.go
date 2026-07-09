package agent

import (
	"fmt"
	"strings"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
	"innovation-incubation-platform-backend/pkg/tokenutil"
)

// buildSystemPrompt 构造 System Prompt。
// 返回值 1: 完整 Prompt（含记忆上下文）；返回值 2: 不含记忆上下文的模板 Token 数。
func buildSystemPrompt(memoryContext string, tools []agenttools.Tool) (string, int) {
	var sb strings.Builder
	sb.WriteString("你是一个创新孵化平台的智能助手，帮助企业和载体用户查询政策、了解入驻状态、追踪诉求进度。")
	sb.WriteString("请使用工具获取最新信息，不确定时如实告知用户。")
	sb.WriteString("用户看不到你的工具调用过程，回复时直接给出结论，不要提及'根据工具返回'等内部机制。")
	sb.WriteString("\n\n")

	if memoryContext != "" {
		sb.WriteString(memoryContext)
		sb.WriteString("\n\n")
	}

	sb.WriteString("可用工具：\n")
	for _, t := range tools {
		fmt.Fprintf(&sb, "- %s：%s\n", t.Name(), t.Description())
	}

	sb.WriteString("\n\n")
	sb.WriteString("每次回复结束后，调用 record_semantic_memory 记录本轮的用户偏好和工具教训。")
	sb.WriteString("lessons 记录工具调用失败的教训，preferences 记录用户明确表达的偏好。无可留空数组。")

	templateLen := tokenutil.Estimate(sb.String())
	return sb.String(), templateLen
}
