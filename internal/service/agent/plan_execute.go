package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	openai "github.com/sashabaranov/go-openai"
)

// buildPlanProgress 生成进度文本（含完整计划和状态标记）
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
		sb.WriteString(fmt.Sprintf("%s Step %d: %s", marker, s.Index, strings.Join(s.Tools, ", ")))
		if s.Desc != "" {
			sb.WriteString(fmt.Sprintf(" — %s", s.Desc))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n请执行当前步骤（➡️ 标记）。\n")
	sb.WriteString("你必须严格按照当前步骤指示执行。只能调用当前步骤中指定的工具，不要调用计划之外的工具。")
	return sb.String()
}

// RunWithPlan 规划后执行模式：先规划再按步骤顺序执行。
func (e *Engine) RunWithPlan(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	// 1. 加载记忆上下文
	memCtx := e.loadMemory(ctx, sessionID, userID, role, userMessage)

	// 2. 规划
	tools := e.tools.ListForRole(role)
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
		return e.Run(ctx, sessionID, userMessage, role, onEvent)
	}
	var planText string
	if len(planResp.Choices) > 0 {
		planText = planResp.Choices[0].Message.Content
	}
	plan, err := parsePlan(planText, e.tools)
	if err != nil {
		slog.Warn("计划解析失败，回退 ReAct", "error", err)
		return e.Run(ctx, sessionID, userMessage, role, onEvent)
	}
	onEvent(SSEEvent{Type: "plan", Data: plan})

	if len(plan.Steps) == 0 {
		return e.Run(ctx, sessionID, userMessage, role, onEvent)
	}

	// 3. 构建执行阶段的 Messages
	systemPrompt, _ := buildSystemPrompt(memCtx, tools)
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

	var records []ChatMessageRecord
	reflectTrigger := false

	// 4. 按计划逐步执行
	for stepIdx := 0; stepIdx < len(plan.Steps); stepIdx++ {
		if ctx.Err() != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "请求超时"}})
			return nil, ctx.Err()
		}

		// 更新进度到 system prompt 位置
		messages[0].Content = systemPrompt + "\n" + buildPlanProgress(plan, stepIdx+1)

		stream, err := e.startChatStream(ctx, messages, openaiTools)
		if err != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "AI 服务暂不可用"}})
			return nil, err
		}
		thinkContent, toolCalls := readStream(stream, onEvent)
		stream.Close()

		if len(toolCalls) == 0 {
			// 没有工具调用，检查是否还有步骤
			if stepIdx+1 < len(plan.Steps) {
				// 步骤未完成但 LLM 没返回工具调用，尝试强制追问
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: fmt.Sprintf("请继续执行计划的第 %d 步。", stepIdx+1),
				})
				stepIdx-- // 重试当前步骤
				continue
			}
			return e.finishReply(thinkContent, records, stepIdx, reflectTrigger, onEvent)
		}

		tcJSON, _ := json.Marshal(toolCalls)
		messages = append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleAssistant, Content: thinkContent, ToolCalls: toolCalls,
		})
		records = append(records, ChatMessageRecord{
			Role: "assistant", Content: thinkContent, ToolCalls: string(tcJSON),
		})
		onEvent(SSEEvent{Type: "tool_call", Data: toolCalls})

		results := e.executeToolCalls(ctx, toolCalls)
		var hit bool
		messages, records, hit = e.observeToolResults(ctx, results, messages, records, onEvent)
		if hit {
			reflectTrigger = true
			// 重新规划剩余步骤
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: fmt.Sprintf("第 %d 步执行失败，请重新制定剩余步骤的替代计划。格式同前：Plan:\n...", stepIdx+1),
			})
			replanResp, replanErr := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
				Model: planModel, Messages: messages,
			})
			if replanErr != nil {
				slog.Warn("重新规划失败", "error", replanErr)
				break
			}
			var replanText string
			if len(replanResp.Choices) > 0 {
				replanText = replanResp.Choices[0].Message.Content
			}
			newPlan, replanErr := parsePlan(replanText, e.tools)
			if replanErr == nil && len(newPlan.Steps) > 0 {
				plan = newPlan
				onEvent(SSEEvent{Type: "replan", Data: plan})
				stepIdx = -1 // 下次循环从 0 开始（步数重置）
				continue
			}
			slog.Warn("重新规划解析失败，终止执行")
			break
		}
	}

	// 5. 所有步骤完成，生成最终回复
	finalResp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: e.promptModel(),
		Messages: append(messages, openai.ChatCompletionMessage{
			Role: openai.ChatMessageRoleUser,
			Content: "所有计划步骤已完成。请基于以上全部信息给出完整、清晰的最终回复。",
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
		StepsUsed: len(plan.Steps) + 1, ReflectTrigger: reflectTrigger,
	}, nil
}
