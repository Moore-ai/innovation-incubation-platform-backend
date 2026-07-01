package agent

import (
	"fmt"
	"strings"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

func buildSystemPrompt(memoryContext string, tools []agenttools.Tool) string {
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
		sb.WriteString(fmt.Sprintf("- %s：%s\n", t.Name(), t.Description()))
	}

	return sb.String()
}
