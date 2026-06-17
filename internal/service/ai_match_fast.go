package service

import "innovation-incubation-platform-backend/internal/model"

func FieldMatchRule(ent *model.Enterprise, policy *model.Policy) string {
	fields := policy.ExtractedFields
	if fields == nil {
		return "unknown"
	}

	industries, _ := fields["applicable_industries"].([]interface{})
	scales, _ := fields["applicable_scales"].([]interface{})
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

// toSlice converts a JSON value to []interface{}, handling both []interface{} and single string.
func toSlice(v interface{}) []interface{} {
	switch val := v.(type) {
	case []interface{}:
		return val
	case string:
		if val != "" {
			return []interface{}{val}
		}
	}
	return nil
}

func containsAny(str string, candidates []interface{}) bool {
	for _, c := range candidates {
		if s, ok := c.(string); ok && s == str {
			return true
		}
	}
	return false
}
