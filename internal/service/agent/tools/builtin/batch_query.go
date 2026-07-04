package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/sync/errgroup"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*BatchQuery)(nil)

type BatchQuery struct {
	registry *agenttools.ToolRegistry
}

func NewBatchQuery(registry *agenttools.ToolRegistry) *BatchQuery {
	return &BatchQuery{registry: registry}
}

func (t *BatchQuery) Name() string      { return "batch_query" }
func (t *BatchQuery) Description() string {
	return "批量并发查询多张表。queries 为数组，每项含 table（表名）及该表的查询参数。"
}
func (t *BatchQuery) AllowedRoles() []string { return []string{"government"} }

func (t *BatchQuery) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"queries":{"type":"array","items":{"type":"object"},"description":"查询列表"}},"required":["queries"]}`)
}

func (t *BatchQuery) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"results":{"type":"array"}}}`)
}

func (t *BatchQuery) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		Queries []map[string]any `json:"queries"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}

	type batchResult struct {
		Table  string `json:"table"`
		Result any    `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}
	results := make([]batchResult, len(input.Queries))

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4) // 最多 4 并发
	for i, q := range input.Queries {
		i, q := i, q
		g.Go(func() error {
			table, _ := q["table"].(string)
			tool, ok := t.registry.Get(table)
			if !ok {
				results[i] = batchResult{Table: table, Error: fmt.Sprintf("未知表: %s", table)}
				return nil
			}
			delete(q, "table")
			args, _ := json.Marshal(q)
			resp, err := tool.Execute(ctx, args)
			if err != nil {
				results[i] = batchResult{Table: table, Error: err.Error()}
				return nil
			}
			var v any
			json.Unmarshal(resp, &v)
			results[i] = batchResult{Table: table, Result: v}
			return nil
		})
	}
	g.Wait()

	b, _ := json.Marshal(map[string]any{"results": results})
	return json.RawMessage(b), nil
}
