package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
)

type SSEEvent struct {
	Type string `json:"type"` // "thinking" | "tool_call" | "tool_result" | "reply" | "error" | "done"
	Data any    `json:"data"`
}

type RunResult struct {
	FinalReply     string
	Messages       []ChatMessageRecord
	StepsUsed      int
	ReflectTrigger bool
}

type ChatMessageRecord struct {
	Role       string // "user" | "assistant" | "tool"
	Content    string
	ToolCallID string
	ToolCalls  string // JSON string of tool_calls array
}

type ctxKey string

const (
	ctxKeyUserID           ctxKey = "user_id"
	ctxKeyRole             ctxKey = "role"
	ctxKeyState            ctxKey = "state"
	ctxKeyExcludeFromMsgID ctxKey = "exclude_from_msg_id"
)

func UserIDFromCtx(ctx context.Context) uint {
	v, _ := ctx.Value(ctxKeyUserID).(uint)
	return v
}

func RoleFromCtx(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRole).(string)
	return v
}

func StateFromCtx(ctx context.Context) map[string]any {
	v, _ := ctx.Value(ctxKeyState).(map[string]any)
	return v
}

func WithUserID(ctx context.Context, id uint) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyRole, role)
}

func WithState(ctx context.Context, state map[string]any) context.Context {
	return context.WithValue(ctx, ctxKeyState, maps.Clone(state))
}

// ExcludeFromMessageID 返回编辑重发场景下需跳过的起始消息 ID，0 = 不跳过。
func ExcludeFromMessageID(ctx context.Context) uint {
	v, _ := ctx.Value(ctxKeyExcludeFromMsgID).(uint)
	return v
}

// WithExcludeFromMessageID 标记编辑重发时需排除的消息起始 ID。
func WithExcludeFromMessageID(ctx context.Context, msgID uint) context.Context {
	return context.WithValue(ctx, ctxKeyExcludeFromMsgID, msgID)
}

// WriteSSEEvent 将 SSEEvent 序列化并写入 SSE 流。
func WriteSSEEvent(w io.Writer, f http.Flusher, evt SSEEvent) {
	data, _ := json.Marshal(evt)
	fmt.Fprintf(w, "data: %s\n\n", data)
	f.Flush()
}
