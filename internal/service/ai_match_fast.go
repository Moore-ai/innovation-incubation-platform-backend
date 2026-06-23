package service

import "innovation-incubation-platform-backend/internal/model"

func FieldMatchRule(ent *model.Enterprise, policy *model.Policy) string {
	fields := policy.ExtractedFields
	if fields == nil {
		return "unknown"
	}

	industries, _ := fields["applicable_industries"].([]any)
	scales, _ := fields["applicable_scales"].([]any)
	regions := toSlice(fields["applicable_region"])

	matched := 0
	total := 0

	if len(industries) > 0 {
		total++
		if containsAny(ent.Industry, industries) {
			matched++
		}
	}
	if len(scales) > 0 {
		total++
		if containsAny(ent.Scale, scales) {
			matched++
		}
	}
	if len(regions) > 0 {
		total++
		if containsAny(ent.Address, regions) {
			matched++
		}
	}

	if total == 0 {
		return "unknown"
	}
	if matched == total {
		return "high"
	}
	if matched > 0 {
		return "partial"
	}
	return "none"
}

// toSlice converts a JSON value to []any, handling both []any and single string.
func toSlice(v any) []any {
	switch val := v.(type) {
	case []any:
		return val
	case string:
		if val != "" {
			return []any{val}
		}
	}
	return nil
}

func containsAny(str string, candidates []any) bool {
	for _, c := range candidates {
		if s, ok := c.(string); ok && s == str {
			return true
		}
	}
	return false
}
