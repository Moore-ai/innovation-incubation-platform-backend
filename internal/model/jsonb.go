package model

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONString 是用于 JSONB 类型的自定义类型
type JSONString []string

func (j *JSONString) Scan(src any) error {
	if src == nil {
		*j = nil
		return nil
	}
	switch v := src.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return nil
	}
}

func (j JSONString) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}
