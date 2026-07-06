package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

type ReflectChecker struct {
	outputSchemas map[string]json.RawMessage // 工具名 → OutputSchema
}

func NewReflectChecker(registry *agenttools.ToolRegistry) *ReflectChecker {
	rc := &ReflectChecker{
		outputSchemas: make(map[string]json.RawMessage),
	}
	for _, t := range registry.All() {
		rc.outputSchemas[t.Name()] = t.OutputSchema()
	}
	return rc
}

// Check 返回 (是否触发反思, 触发原因)
func (r *ReflectChecker) Check(_ context.Context, toolName string, result json.RawMessage, execErr error) (bool, string) {
	// 第一层：硬规则
	if execErr != nil {
		return true, fmt.Sprintf("工具执行出错: %v", execErr)
	}
	if len(result) == 0 || string(result) == "null" || string(result) == `""` {
		return true, "工具返回为空"
	}

	s := string(result)
	for _, kw := range []string{"权限不足", "无权限", "forbidden", "unauthorized", "内部错误", "服务不可用"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(kw)) {
			return true, fmt.Sprintf("工具返回包含异常关键词: %s", kw)
		}
	}

	// 第二层：合法 JSON 校验
	if !json.Valid(result) {
		return true, "工具返回不是合法的 JSON"
	}

	// 第三层：结构校验（OutputSchema declared key 存在且类型匹配）
	schema, ok := r.outputSchemas[toolName]
	if ok {
		if reason := validateStructure(result, schema); reason != "" {
			return true, reason
		}
	}

	return false, ""
}

// validateStructure 校验 result JSON 中 declared 的 key 是否存在且类型匹配。
func validateStructure(result, schema json.RawMessage) string {
	var res map[string]any
	if err := json.Unmarshal(result, &res); err != nil {
		return fmt.Sprintf("结果 JSON 解析失败: %v", err)
	}

	var sch struct {
		Properties map[string]struct {
			Type     string   `json:"type"`
			Required []string `json:"required"`
		} `json:"properties"`
		Required []string `json:"required"`
	}
	if err := json.Unmarshal(schema, &sch); err != nil || len(sch.Properties) == 0 {
		return "" // schema 不完整则跳过
	}

	for key, prop := range sch.Properties {
		v, exists := res[key]
		if !exists {
			// 只在 required 列表中才报错
			if slices.Contains(sch.Required, key) {
				return fmt.Sprintf("缺少必要字段: %s", key)
			}
			continue
		}

		propType := strings.ToLower(prop.Type)
		if !matchType(v, propType) {
			return fmt.Sprintf("字段 %s 类型不匹配: 期望 %s, 实际 %T", key, propType, v)
		}
	}
	return ""
}

// matchType 检查值的 JSON 类型是否与 schema type 匹配。
func matchType(v any, schematype string) bool {
	switch schematype {
	case "string":
		_, ok := v.(string)
		return ok
	case "integer", "number":
		switch v.(type) {
		case float64, int, int64, json.Number:
			return true
		default:
			return false
		}
	case "array":
		_, ok := v.([]any)
		return ok
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	default:
		return true
	}
}
