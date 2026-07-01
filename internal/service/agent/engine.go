package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
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

// Run 执行 ReAct 循环。onEvent 回调推送 SSE 事件。
func (e *Engine) Run(ctx context.Context, sessionID uint, userMessage string, role string, onEvent func(SSEEvent)) (*RunResult, error) {
	userID := UserIDFromCtx(ctx)
	if userID == 0 {
		return nil, fmt.Errorf("user_id not found in context")
	}

	// 加载记忆上下文
	memCtx, err := e.memory.LoadContext(ctx, sessionID, userID, userMessage)
	if err != nil {
		slog.Warn("加载记忆上下文失败", "error", err, "session_id", sessionID)
	}

	// 过滤工具列表
	tools := e.tools.ListForRole(role)

	// System Prompt
	systemPrompt := buildSystemPrompt(memCtx, tools)

	// 构建 OpenAI Tools 参数
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

	// 消息历史
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: userMessage},
	}

	var records []ChatMessageRecord
	reflectTrigger := false
	var finalReply string

	for step := 0; step < e.cfg.MaxSteps; step++ {
		// 检查全局超时
		if ctx.Err() != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "请求超时，请重试"}})
			return nil, ctx.Err()
		}

		// Think: 调用 LLM（流式）
		model := e.cfg.Model
		if model == "" {
			model = e.ai.Model() // 沿用现有模型名
		}
		req := openai.ChatCompletionRequest{
			Model:    model,
			Messages: messages,
			Tools:    openaiTools,
			Stream:   true,
		}

		stream, err := e.ai.CreateChatCompletionStream(ctx, req)
		if err != nil {
			onEvent(SSEEvent{Type: "error", Data: map[string]string{"message": "AI 服务暂不可用，请稍后重试"}})
			return nil, err
		}

		// 读取流式响应 -> 收集 content + tool_calls
		var thinkContent strings.Builder
		var toolCalls []openai.ToolCall
		for {
			recv, recvErr := stream.Recv()
			if recvErr != nil {
				break
			}
			for _, choice := range recv.Choices {
				thinkContent.WriteString(choice.Delta.Content)
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
		// 关闭流（不使用 defer 避免循环内累积）
		stream.Close()

		if len(toolCalls) > 0 {
			// Assistant 消息（含 tool_calls）
			tcJSON, _ := json.Marshal(toolCalls)
			assistantMsg := openai.ChatCompletionMessage{
				Role:      openai.ChatMessageRoleAssistant,
				Content:   thinkContent.String(),
				ToolCalls: toolCalls,
			}
			messages = append(messages, assistantMsg)
			records = append(records, ChatMessageRecord{
				Role:      "assistant",
				Content:   thinkContent.String(),
				ToolCalls: string(tcJSON),
			})

			// Act: 并行执行工具
			onEvent(SSEEvent{Type: "tool_call", Data: toolCalls})

			type toolResult struct {
				callID  string
				name    string
				content json.RawMessage
				err     error
			}
			results := make(chan toolResult, len(toolCalls))

			sem := make(chan struct{}, 4)
			g, gctx := errgroup.WithContext(ctx)
			for _, tc := range toolCalls {
				g.Go(func() error {
					sem <- struct{}{}
					defer func() { <-sem }()

					// 工具执行超时
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

			// Observe: 处理工具结果
			for res := range results {
				// 脱敏
				if res.content != nil {
					res.content = maskSensitive(res.content)
				}

				var content string
				if res.err != nil {
					content = fmt.Sprintf("error: %v", res.err)
				} else {
					content = string(res.content)
				}

				toolMsg := openai.ChatCompletionMessage{
					Role:       openai.ChatMessageRoleTool,
					Content:    content,
					ToolCallID: res.callID,
				}
				messages = append(messages, toolMsg)
				records = append(records, ChatMessageRecord{
					Role:       "tool",
					Content:    content,
					ToolCallID: res.callID,
				})

				// Reflect 检查
				hit, reason := e.reflect.Check(ctx, res.name, res.content, res.err)
				if hit {
					reflectTrigger = true
					onEvent(SSEEvent{Type: "tool_result", Data: map[string]any{
						"tool": res.name, "result": content, "reflect": true, "reason": reason,
					}})
					// 注入反思 Prompt
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
		} else {
			// 纯文本回复 → 最终输出
			finalReply = thinkContent.String()
			records = append(records, ChatMessageRecord{
				Role:    "assistant",
				Content: finalReply,
			})
			onEvent(SSEEvent{Type: "reply", Data: finalReply})
			onEvent(SSEEvent{Type: "done", Data: nil})
			return &RunResult{
				FinalReply:     finalReply,
				Messages:       records,
				StepsUsed:      step + 1,
				ReflectTrigger: reflectTrigger,
			}, nil
		}
	}

	// 达到 MaxSteps：注入最终指令
	messages = append(messages, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: "你已达最大步数限制，请基于已有的观察信息给出当前能提供的最佳回答，无需继续调用工具。",
	})
	// 最后一次 LLM 调用（非流式，简单处理）
	model := e.cfg.Model
	if model == "" {
		model = e.ai.Model()
	}
	resp, err := e.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	})
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
