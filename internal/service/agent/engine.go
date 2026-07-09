package agent

import (
	"context"
	"encoding/json"
	"fmt"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/aiclient"
	"innovation-incubation-platform-backend/pkg/tokenutil"

	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

type Engine struct {
	ai      *aiclient.Client
	tools   *agenttools.ToolRegistry
	memory  *agentmemory.MemoryManager
	reflect *ReflectChecker
	cfg     config.AgentConfig

	roleToolTokens map[string]int // 按角色预缓存的工具定义 Token 数
}

func NewEngine(ai *aiclient.Client, tools *agenttools.ToolRegistry, mem *agentmemory.MemoryManager, reflect *ReflectChecker, cfg config.AgentConfig) *Engine {
	tokenutil.SetEstimationMode(cfg.TokenEstimation)

	// 按角色预计算工具定义 Token
	roleToolTokens := make(map[string]int)
	for _, role := range []string{"enterprise", "carrier", "government"} {
		roleTools := tools.ListForRole(role)
		roleToolTokens[role] = calcToolDefTokens(roleTools)
	}

	return &Engine{
		ai:             ai,
		tools:          tools,
		memory:         mem,
		reflect:        reflect,
		cfg:            cfg,
		roleToolTokens: roleToolTokens,
	}
}

type toolResult struct {
	callID  string
	name    string
	content json.RawMessage
	err     error
}

// streamReader 可供测试 mock 的流式响应接口。
type streamReader interface {
	Recv() (openai.ChatCompletionStreamResponse, error)
	Close() error
}

// readStream 读取流式 LLM 响应，收集 content（增量 SSE 推送）+ tool_calls（index 合并）。
func readStream(stream streamReader, onEvent func(SSEEvent)) (content string, toolCalls []openai.ToolCall) {
	for {
		recv, err := stream.Recv()
		if err != nil {
			break
		}
		for _, choice := range recv.Choices {
			content += choice.Delta.Content
			if len(choice.Delta.Content) > 0 {
				onEvent(SSEEvent{Type: "thinking", Data: choice.Delta.Content})
			}
			for _, tc := range choice.Delta.ToolCalls {
				if tc.Index != nil {
					idx := *tc.Index
					for len(toolCalls) <= idx {
						toolCalls = append(toolCalls, openai.ToolCall{})
					}
					if tc.ID != "" {
						toolCalls[idx].ID = tc.ID
					}
					if tc.Type != "" {
						toolCalls[idx].Type = tc.Type
					}
					if tc.Function.Name != "" {
						toolCalls[idx].Function.Name += tc.Function.Name
					}
					toolCalls[idx].Function.Arguments += tc.Function.Arguments
				}
			}
		}
	}
	return content, toolCalls
}

// executeToolCalls 并行执行所有 tool_calls（errgroup + 信号量限制并发）。
func (e *Engine) executeToolCalls(ctx context.Context, calls []openai.ToolCall) []toolResult {
	results := make(chan toolResult, len(calls))

	sem := make(chan struct{}, 4)
	g, gctx := errgroup.WithContext(ctx)
	for _, tc := range calls {
		g.Go(func() error {
			sem <- struct{}{}
			defer func() { <-sem }()

			tool, ok := e.tools.Get(tc.Function.Name)
			if !ok {
				results <- toolResult{callID: tc.ID, name: tc.Function.Name, err: fmt.Errorf("tool %s not found", tc.Function.Name)}
				return nil
			}

			tctx, cancel := context.WithTimeout(gctx, tool.Timeout())
			defer cancel()
			raw, execErr := tool.Execute(tctx, json.RawMessage(tc.Function.Arguments))
			results <- toolResult{callID: tc.ID, name: tc.Function.Name, content: raw, err: execErr}
			return nil
		})
	}
	g.Wait()
	close(results)

	out := make([]toolResult, 0, len(calls))
	for r := range results {
		out = append(out, r)
	}
	return out
}

// observeToolResults 处理工具执行结果：脱敏 → 追加消息 → Reflect 检查 → 注入反思 Prompt。
func (e *Engine) observeToolResults(
	ctx context.Context,
	results []toolResult,
	messages []openai.ChatCompletionMessage,
	records []ChatMessageRecord,
	onEvent func(SSEEvent),
) ([]openai.ChatCompletionMessage, []ChatMessageRecord, bool) {
	reflectTrigger := false
	for _, res := range results {
		if res.content != nil {
			res.content = maskSensitive(res.content)
		}
		var content string
		if res.err != nil {
			content = fmt.Sprintf("error: %v", res.err)
		} else {
			content = string(res.content)
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:       openai.ChatMessageRoleTool,
			Content:    content,
			ToolCallID: res.callID,
		})
		records = append(records, ChatMessageRecord{
			Role:       "tool",
			Content:    content,
			ToolCallID: res.callID,
		})

		hit, reason := e.reflect.Check(ctx, res.name, res.content, res.err)
		if hit {
			reflectTrigger = true
			onEvent(SSEEvent{Type: "tool_result", Data: map[string]any{
				"tool": res.name, "result": content, "reflect": true, "reason": reason,
			}})
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("上一步工具调用 `%s` 出现问题: %s。请分析可能的原因，调整策略，尝试其他方法或参数后重新执行。", res.name, reason),
			})
		} else {
			onEvent(SSEEvent{Type: "tool_result", Data: map[string]any{
				"tool": res.name, "result": content, "reflect": false,
			}})
		}
	}
	return messages, records, reflectTrigger
}

// promptModel 返回使用的模型名。
func (e *Engine) promptModel() string {
	if e.cfg.Model != "" {
		return e.cfg.Model
	}
	return e.ai.Model()
}

// startChatStream 发起流式 LLM 调用。
func (e *Engine) startChatStream(ctx context.Context, messages []openai.ChatCompletionMessage, openaiTools []openai.Tool) (*openai.ChatCompletionStream, error) {
	return e.ai.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    e.promptModel(),
		Messages: messages,
		Tools:    openaiTools,
		Stream:   true,
	})
}

// finishReply 构造纯文本回复的 RunResult。
func (e *Engine) finishReply(thinkContent string, records []ChatMessageRecord, step int, reflectTrigger bool, onEvent func(SSEEvent)) (*RunResult, error) {
	records = append(records, ChatMessageRecord{
		Role:    "assistant",
		Content: thinkContent,
	})
	onEvent(SSEEvent{Type: "reply", Data: thinkContent})
	onEvent(SSEEvent{Type: "done", Data: nil})
	return &RunResult{
		FinalReply:     thinkContent,
		Messages:       records,
		StepsUsed:      step + 1,
		ReflectTrigger: reflectTrigger,
	}, nil
}

// filterInternalEvents 过滤掉工具调用等内部事件，只保留配置中允许的类型。
func (e *Engine) filterInternalEvents(onEvent func(SSEEvent)) func(SSEEvent) {
	allowed := e.cfg.PublicSSETypes
	allowSet := make(map[string]bool, len(allowed))
	for _, t := range allowed {
		allowSet[t] = true
	}
	return func(evt SSEEvent) {
		if allowSet[evt.Type] {
			onEvent(evt)
		}
	}
}

// calcToolDefTokens 计算工具定义序列化为 OpenAI Tool 后的近似 Token 总数。
func calcToolDefTokens(tools []agenttools.Tool) int {
	total := 0
	for _, t := range tools {
		ot := openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		}
		b, err := json.Marshal(ot)
		if err != nil {
			continue
		}
		total += tokenutil.Estimate(string(b))
	}
	return total
}
