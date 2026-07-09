package tools

import (
	"context"
	"encoding/json"
	"time"
)

type Tool interface {
	Name()         string
	Description()  string
	AllowedRoles() []string
	InputSchema()  json.RawMessage
	OutputSchema() json.RawMessage
	Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error)
	Timeout()     time.Duration
}

func DefaultTimeout() time.Duration { return 15 * time.Second }

type ctxKey string

const CtxKeyUserID ctxKey = "tool_user_id"

func UserIDFromCtx(ctx context.Context) uint {
	v, _ := ctx.Value(CtxKeyUserID).(uint)
	return v
}
