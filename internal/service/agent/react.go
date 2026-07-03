package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	openai "github.com/sashabaranov/go-openai"
)

// Run 执行 ReAct 循环。onEvent 回调推送 SSE 事件。
func (e *Engine) Run(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	if e.cfg.PlanningEnabled {
		return e.RunWithPlan(ctx, sessionID, userMessage, role, onEvent)
	}
	return e.runReAct(ctx, sessionID, userMessage, role, onEvent)
}

// runReAct 执行 ReAct 循环（不检查 PlanningEnabled）。
func (e *Engine) runReAct(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	onEvent = e.filterInternalEvents(onEvent)
	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	tools := e.tools.ListForRole(role)
	memCtx := e.loadMemory(ctx, sessionID, userID, role, userMessage)

	systemPrompt, _ := buildSystemPrompt(memCtx, tools)
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
		tcJSON, err := json.Marshal(toolCalls)
		if err != nil {
			slog.Warn("工具调用序列化失败", "error", err)
			tcJSON = []byte("[]")
		}
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

// loadMemory 加载记忆上下文，返回拼接好的记忆文本。
func (e *Engine) loadMemory(ctx context.Context, sessionID, userID uint, role, query string) string {
	tools := e.tools.ListForRole(role)
	_, templateTokens := buildSystemPrompt("", tools)
	toolDefTokens, ok := e.roleToolTokens[role]
	if !ok {
		toolDefTokens = calcToolDefTokens(tools)
	}
	budget := int(float64(e.cfg.ContextWindow)*e.cfg.HistoryBudgetRatio) - toolDefTokens - templateTokens
	if budget <= 0 {
		slog.Warn("历史消息预算为0或负数，跳过所有记忆加载", "budget", budget, "session_id", sessionID)
	}
	memCtx, err := e.memory.LoadContext(ctx, sessionID, userID, query, budget)
	if err != nil {
		slog.Warn("加载记忆上下文失败", "error", err, "session_id", sessionID)
	}
	return memCtx
}
