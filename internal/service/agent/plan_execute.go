package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

const maxStepRetries = 3

func buildPlanProgress(plan *Plan, currentStep int) string {
	var sb strings.Builder
	sb.WriteString("计划执行进度：\n")
	for i, s := range plan.Steps {
		marker := "⏸️"
		if i < currentStep-1 {
			marker = "✅"
		} else if i == currentStep-1 {
			marker = "➡️"
		}
		fmt.Fprintf(&sb, "%s Step %d: %s", marker, s.Index, strings.Join(s.Tools, ", "))
		if s.Desc != "" {
			fmt.Fprintf(&sb, " — %s", s.Desc)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n请执行当前步骤（➡️ 标记）。\n")
	sb.WriteString("你必须严格按照当前步骤指示执行。只能调用当前步骤中指定的工具，不要调用计划之外的工具。")
	return sb.String()
}

func (e *Engine) planPhase(ctx context.Context, userMessage string, tools []agenttools.Tool, onEvent func(SSEEvent)) *Plan {
	planPrompt := buildPlanPrompt(tools)
	planModel := e.cfg.PlanningModel
	if planModel == "" {
		planModel = e.promptModel()
	}
	planResp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: planModel,
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: planPrompt},
			{Role: openai.ChatMessageRoleUser, Content: userMessage},
		},
	})
	if err != nil {
		slog.Warn("规划调用失败，回退 ReAct", "error", err)
		onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "规划失败，使用标准模式回复"}})
		return nil
	}
	var planText string
	if len(planResp.Choices) > 0 {
		planText = planResp.Choices[0].Message.Content
	}
	plan, err := parsePlan(planText, e.tools)
	if err != nil {
		slog.Warn("计划解析失败，回退 ReAct", "error", err)
		onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "规划失败，使用标准模式回复"}})
		return nil
	}
	return plan
}

func (e *Engine) buildExecContext(ctx context.Context, memCtx string, tools []agenttools.Tool, plan *Plan, userMessage string) (string, []openai.ChatCompletionMessage, []openai.Tool) {
	systemPrompt, _ := buildSystemPrompt(memCtx, tools)
	if stateCtx := formatStateContext(StateFromCtx(ctx)); stateCtx != "" {
		systemPrompt += "\n" + stateCtx
	}
	systemPrompt += "\n" + buildPlanProgress(plan, 0)
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMessage},
	}
	openaiTools := make([]openai.Tool, 0, len(tools))
	for _, t := range tools {
		openaiTools = append(openaiTools, openai.Tool{
			Type: openai.ToolTypeFunction,
			Function: &openai.FunctionDefinition{
				Name: t.Name(), Description: t.Description(), Parameters: t.InputSchema(),
			},
		})
	}
	return systemPrompt, messages, openaiTools
}

func (e *Engine) replanPhase(ctx context.Context, messages []openai.ChatCompletionMessage, planModel string, onEvent func(SSEEvent)) (newPlan *Plan, replanText string) {
	formatHint := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleSystem,
		Content: buildReplanFormatHint(e.tools.All()),
	}
	hintedMessages := append([]openai.ChatCompletionMessage{formatHint}, messages[1:]...)

	resp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{Model: planModel, Messages: hintedMessages})
	if err != nil {
		slog.Warn("重新规划失败", "error", err)
		return nil, ""
	}
	if len(resp.Choices) > 0 {
		replanText = resp.Choices[0].Message.Content
	}
	newPlan, err = parsePlan(replanText, e.tools)
	if err != nil || len(newPlan.Steps) == 0 {
		slog.Warn("重新规划解析失败，终止执行", "text", replanText[:min(len(replanText), 200)])
		return nil, ""
	}
	onEvent(SSEEvent{Type: "replan", Data: newPlan})
	return newPlan, replanText
}

func (e *Engine) runExecStep(ctx context.Context, messages []openai.ChatCompletionMessage, openaiTools []openai.Tool, onEvent func(SSEEvent)) (string, []openai.ToolCall, error) {
	stream, err := e.startChatStream(ctx, messages, openaiTools)
	if err != nil {
		return "", nil, err
	}
	defer stream.Close()
	thinkContent, toolCalls := readStream(stream, onEvent)
	return thinkContent, toolCalls, nil
}

func (e *Engine) RunWithPlan(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	onEvent = e.filterInternalEvents(onEvent)

	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	memCtx := e.loadMemory(ctx, sessionID, userID, role, userMessage)
	tools := e.tools.ListForRole(role)

	plan := e.planPhase(ctx, userMessage, tools, onEvent)
	if plan == nil || len(plan.Steps) == 0 {
		return e.runReAct(ctx, sessionID, userMessage, role, onEvent)
	}
	onEvent(SSEEvent{Type: "plan", Data: plan})

	systemPrompt, messages, openaiTools := e.buildExecContext(ctx, memCtx, tools, plan, userMessage)
	planModel := e.cfg.PlanningModel
	if planModel == "" {
		planModel = e.promptModel()
	}

	var records []ChatMessageRecord
	reflectTrigger, retries, replanFailed, completedSteps := false, 0, false, 0

	for stepIdx := 0; stepIdx < len(plan.Steps); stepIdx++ {
		if ctx.Err() != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "请求超时"}})
			return nil, ctx.Err()
		}

		messages[0].Content = systemPrompt + "\n" + buildPlanProgress(plan, stepIdx+1)
		thinkContent, toolCalls, err := e.runExecStep(ctx, messages, openaiTools, onEvent)
		if err != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "AI 服务暂不可用"}})
			return nil, err
		}

		if len(toolCalls) == 0 {
			if stepIdx+1 < len(plan.Steps) {
				retries++
				if retries <= maxStepRetries {
					messages = append(messages, openai.ChatCompletionMessage{
						Role:    openai.ChatMessageRoleUser,
						Content: fmt.Sprintf("请继续执行计划的第 %d 步。", stepIdx+1),
					})
					stepIdx--
				} else {
					slog.Warn("步骤重试次数超限", "step", stepIdx+1)
				}
				continue
			}
			onEvent(SSEEvent{Type: "reply", Data: thinkContent})
			return e.finishReply(thinkContent, records, completedSteps, reflectTrigger, onEvent)
		}

		tcJSON, err := json.Marshal(toolCalls)
		if err != nil {
			slog.Warn("工具调用序列化失败", "error", err)
			tcJSON = []byte("[]")
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant, Content: thinkContent, ToolCalls: toolCalls,
		})
		records = append(records, ChatMessageRecord{
			Role: "assistant", Content: thinkContent, ToolCalls: string(tcJSON),
		})
		onEvent(SSEEvent{Type: "tool_call", Data: toolCalls})

		execCtx := WithProgressWriter(ctx, func(typ string, data map[string]any) {
			onEvent(SSEEvent{Type: typ, Data: data})
		})
		// 注入 userID 供 record_semantic_memory 等工具读取
		execCtx = context.WithValue(execCtx, agenttools.CtxKeyUserID, userID)
		results := e.executeToolCalls(execCtx, toolCalls)
		var hit bool
		messages, records, hit = e.observeToolResults(ctx, results, messages, records, onEvent)
		completedSteps++
		if !hit {
			continue
		}

		reflectTrigger = true
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("第 %d 步执行失败。请立即输出替代计划，只输出计划本身（不要其他文字）。格式如下：\n\nPlan:\n1. tool(param) — 说明\n", stepIdx+1),
		})
		newPlan, replanText := e.replanPhase(ctx, messages, planModel, onEvent)
		if newPlan == nil {
			replanFailed = true
			break
		}
		messages = append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant, Content: replanText,
		})
		records = append(records, ChatMessageRecord{Role: "assistant", Content: replanText})
		plan = newPlan
		systemPrompt, _ = buildSystemPrompt(memCtx, tools)
		if stateCtx := formatStateContext(StateFromCtx(ctx)); stateCtx != "" {
			systemPrompt += "\n" + stateCtx
		}
		stepIdx = -1
	}

	promptText := "所有计划步骤已完成。请基于以上全部信息给出完整、清晰的最终回复。"
	if replanFailed {
		promptText = "部分计划步骤未能完成。请基于已有的信息给出当前能提供的最佳回复。"
	}
	finalResp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: e.promptModel(),
		Messages: append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleUser, Content: promptText,
		}),
	})
	var finalReply string
	if err != nil {
		finalReply = "抱歉，最终回复生成失败，请根据以上信息自行判断。"
	} else if len(finalResp.Choices) > 0 {
		finalReply = finalResp.Choices[0].Message.Content
	}
	records = append(records, ChatMessageRecord{Role: "assistant", Content: finalReply})
	onEvent(SSEEvent{Type: "reply", Data: finalReply})
	onEvent(SSEEvent{Type: "done", Data: nil})
	return &RunResult{
		FinalReply: finalReply, Messages: records,
		StepsUsed: completedSteps + 1, ReflectTrigger: reflectTrigger,
	}, nil
}
