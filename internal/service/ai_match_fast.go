package service

import "innovation-incubation-platform-backend/internal/model"

func FieldMatchRule(ent *model.Enterprise, policy *model.Policy) string {
	fields := policy.ExtractedFields
	if fields == nil {
		return "unknown"
	}

	industries, _ := fields["applicable_industries"].([]interface{})
	scales, _ := fields["applicable_scales"].([]interface{})
	regions, _ := fields["applicable_region"].([]interface{})

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

func containsAny(str string, candidates []interface{}) bool {
	for _, c := range candidates {
		if s, ok := c.(string); ok && s == str {
			return true
		}
	}
	return false
}
