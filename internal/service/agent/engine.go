package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/pkg/aiclient"

	agentmemory "innovation-incubation-platform-backend/internal/service/agent/memory"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

type Engine struct {
	ai      *aiclient.Client
	tools   *agenttools.ToolRegistry
	memory  *agentmemory.MemoryManager
	reflect *ReflectChecker
	cfg     config.AgentConfig
}

func NewEngine(ai *aiclient.Client, tools *agenttools.ToolRegistry, mem *agentmemory.MemoryManager, reflect *ReflectChecker, cfg config.AgentConfig) *Engine {
	return &Engine{ai: ai, tools: tools, memory: mem, reflect: reflect, cfg: cfg}
}

type toolResult struct {
	callID  string
	name    string
	content json.RawMessage
	err     error
}

// readStream 读取流式 LLM 响应，收集 content（增量 SSE 推送）+ tool_calls（index 合并）。
func readStream(stream *openai.ChatCompletionStream, onEvent func(SSEEvent)) (content string, toolCalls []openai.ToolCall) {
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

			tctx, cancel := context.WithTimeout(gctx, time.Duration(e.cfg.ToolTimeoutSec)*time.Second)
			defer cancel()

			tool, ok := e.tools.Get(tc.Function.Name)
			if !ok {
				results <- toolResult{callID: tc.ID, name: tc.Function.Name, err: fmt.Errorf("tool %s not found", tc.Function.Name)}
				return nil
			}
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

// Run 执行 ReAct 循环。onEvent 回调推送 SSE 事件。
func (e *Engine) Run(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	memCtx, err := e.memory.LoadContext(ctx, sessionID, userID, userMessage)
	if err != nil {
		slog.Warn("加载记忆上下文失败", "error", err, "session_id", sessionID)
	}

	tools := e.tools.ListForRole(role)
	systemPrompt := buildSystemPrompt(memCtx, tools)
	openaiTools := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		openaiTools = append(openaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.InputSchema(),
			},
		})
	}

	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMessage},
	}
	var records []ChatMessageRecord
	reflectTrigger := false

	for step := 0; step < e.cfg.MaxSteps; step++ {
		if ctx.Err() != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "请求超时，请重试"}})
			return nil, ctx.Err()
		}

		// Think: 流式调用 LLM
		stream, err := e.startChatStream(ctx, messages, openaiTools)
		if err != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "AI 服务暂不可用，请稍后重试"}})
			return nil, err
		}
		thinkContent, toolCalls := readStream(stream, onEvent)
		stream.Close()

		if len(toolCalls) == 0 {
			return e.finishReply(thinkContent, records, step, reflectTrigger, onEvent)
		}

		// 记录 Assistant 消息
		tcJSON, _ := json.Marshal(toolCalls)
		messages = append(messages, openai.ChatCompletionMessage{
			Role:      openai.ChatMessageRoleAssistant,
			Content:   thinkContent,
			ToolCalls: toolCalls,
		})
		records = append(records, ChatMessageRecord{
			Role:      "assistant",
			Content:   thinkContent,
			ToolCalls: string(tcJSON),
		})

		onEvent(SSEEvent{Type: "tool_call", Data: toolCalls})

		// Act: 并行执行工具
		results := e.executeToolCalls(ctx, toolCalls)

		// Observe: 处理工具结果
		var hit bool
		messages, records, hit = e.observeToolResults(ctx, results, messages, records, onEvent)
		if hit {
			reflectTrigger = true
		}
	}

	// 达到 MaxSteps：注入最终指令
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "你已达最大步数限制，请基于已有的观察信息给出当前能提供的最佳回答，无需继续调用工具。",
	})
	resp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    e.promptModel(),
		Messages: messages,
	})
	var finalReply string
	if err != nil {
		finalReply = "抱歉，查询较为复杂，请尝试简化问题后重新提问。"
	} else if len(resp.Choices) > 0 {
		finalReply = resp.Choices[0].Message.Content
	} else {
		finalReply = "抱歉，未能完成查询，请稍后重试。"
	}
	records = append(records, ChatMessageRecord{
		Role:    "assistant",
		Content: finalReply,
	})
	onEvent(SSEEvent{Type: "reply", Data: finalReply})
	onEvent(SSEEvent{Type: "done", Data: nil})

	return &RunResult{
		FinalReply:     finalReply,
		Messages:       records,
		StepsUsed:      e.cfg.MaxSteps,
		ReflectTrigger: reflectTrigger,
	}, nil
}
