package agent

import (
	"fmt"
	"strings"
)

// formatStateContext 将前端传递的临时状态格式化为 LLM 可读的上下文文本。
// nil 或空 map 返回空字符串。
func formatStateContext(state map[string]any) string {
	if len(state) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 当前用户状态\n")
	for k, v := range state {
		sv := fmt.Sprintf("%v", v)
		sv = strings.ReplaceAll(sv, "\n", "\\n")
		fmt.Fprintf(&sb, "- %s: %s\n", k, sv)
	}
	return sb.String()
}
