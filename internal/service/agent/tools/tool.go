package tools

import (
	"context"
	"encoding/json"
)

type Tool interface {
	Name()         string
	Description()  string
	AllowedRoles() []string
	InputSchema()  json.RawMessage
	OutputSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
}
