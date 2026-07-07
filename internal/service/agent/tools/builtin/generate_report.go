package builtin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"

	openai "github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/storage"
	"innovation-incubation-platform-backend/pkg/aiclient"

	agent "innovation-incubation-platform-backend/internal/service/agent"
	agenttools "innovation-incubation-platform-backend/internal/service/agent/tools"
)

var _ agenttools.Tool = (*GenerateReport)(nil)

type GenerateReport struct {
	ai          *aiclient.Client
	db          *gorm.DB
	engine      *QueryEngine
	configs     []*TableConfig
	converter   *ReportConverter
	fileRepo    *repository.FileRepo
	fileStorage storage.Storage
}

func NewGenerateReport(ai *aiclient.Client, db *gorm.DB, converter *ReportConverter, fileRepo *repository.FileRepo, fileStorage storage.Storage) *GenerateReport {
	return &GenerateReport{ai: ai, db: db, engine: &QueryEngine{}, configs: TableConfigs, converter: converter, fileRepo: fileRepo, fileStorage: fileStorage}
}

func (t *GenerateReport) Name() string           { return "generate_report" }
func (t *GenerateReport) AllowedRoles() []string { return []string{"government"} }
func (t *GenerateReport) Timeout() time.Duration { return 180 * time.Second }
func (t *GenerateReport) Description() string {
	return "生成数据分析报告（PDF 或 DOCX 格式，含图表）。如用户未指定格式，请主动询问。"
}

func (t *GenerateReport) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"prompt":{"type":"string","description":"报告主题和要求"},"format":{"type":"string","enum":["pdf","docx"],"description":"输出格式"}},"required":["prompt","format"]}`)
}

func (t *GenerateReport) OutputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"file_url":{"type":"string"},"format":{"type":"string"}}}`)
}

func (t *GenerateReport) Execute(ctx context.Context, args json.RawMessage) (json.RawMessage, error) {
	var input struct {
		Prompt string `json:"prompt"`
		Format string `json:"format"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return nil, err
	}
	if input.Format == "" {
		input.Format = "pdf"
	}

	pw := agent.ProgressWriterFromCtx(ctx)

	// Phase 1: Analyst
	sendProgress(pw, "report_start", map[string]any{"phase": "analyst"})
	plans, err := t.runAnalyst(ctx, input.Prompt)
	if err != nil {
		return nil, fmt.Errorf("分析阶段失败: %w", err)
	}
	if len(plans) == 0 {
		return nil, fmt.Errorf("分析师未规划任何图表，请细化需求后重试")
	}

	// Phase 2: Executor (并发)
	charts, err := t.runExecutor(ctx, plans, pw)
	if err != nil {
		return nil, fmt.Errorf("执行阶段失败: %w", err)
	}

	// Phase 3: Summarizer
	sendProgress(pw, "report_progress", map[string]any{"phase": "summarizer"})
	markdown, err := t.runSummarizer(ctx, input.Prompt, charts)
	if err != nil {
		return nil, fmt.Errorf("汇总阶段失败: %w", err)
	}

	markdown = cleanMarkdown(markdown)

	// Phase 4: Converter
	sendProgress(pw, "report_progress", map[string]any{"phase": "converter"})
	fileURL, err := t.runConverter(ctx, markdown, input.Format, pw)
	if err != nil {
		return nil, fmt.Errorf("格式转换失败: %w", err)
	}

	sendProgress(pw, "report_done", map[string]any{"file_url": fileURL, "format": input.Format})
	result, _ := json.Marshal(map[string]string{"file_url": fileURL, "format": input.Format})
	return json.RawMessage(result), nil
}

// --- Phase 1: Analyst ---

func (t *GenerateReport) runAnalyst(ctx context.Context, prompt string) ([]ChartPlan, error) {
	plans, err := chatAndParse[[]ChartPlan](t.ai, ctx, analystPrompt(t.buildTableList()), prompt, "分析阶段解析失败")
	if err != nil {
		return nil, err
	}
	if plans == nil {
		return nil, nil
	}
	return *plans, nil
}

func (t *GenerateReport) buildTableList() string {
	var sb strings.Builder
	for _, c := range t.configs {
		sb.WriteString("- ")
		sb.WriteString(c.TblName())
		sb.WriteString(": ")
		sb.WriteString(c.Description)
		sb.WriteString("\n")
	}
	return sb.String()
}

// --- Phase 2: Executor (并发，无 AI 调用) ---

func (t *GenerateReport) runExecutor(ctx context.Context, plans []ChartPlan, pw agent.ProgressWriter) ([]ReportChart, error) {
	results := make([]ReportChart, len(plans))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(4)

	var mu sync.Mutex
	doneCount := 0

	for i, plan := range plans {
		g.Go(func() error {
			var cfg *TableConfig
			for _, c := range t.configs {
				if c.TblName() == plan.Table {
					cfg = c
					break
				}
			}
			if cfg == nil {
				return fmt.Errorf("未知表[%s]: %s", plan.Title, plan.Table)
			}

			qr, err := t.engine.Query(t.db, cfg, plan.Params)
			if err != nil {
				return fmt.Errorf("查询失败[%s]: %w", plan.Title, err)
			}

			var mermaid string
			if plan.Type == "flowchart" {
				spec := ChartSpec{Type: plan.Type, Title: plan.Title, XLabel: plan.XLabel, YLabel: plan.YLabel, Colors: plan.Colors}
				mermaid, err = buildMermaidFlowchart(t.ai, ctx, spec, qr)
				if err != nil {
					return fmt.Errorf("流程图生成失败[%s]: %w", plan.Title, err)
				}
			} else {
				mermaid = buildMermaidForPlan(plan, qr)
			}

			results[i] = ReportChart{
				Spec:    ChartSpec{Type: plan.Type, Title: plan.Title, XLabel: plan.XLabel, YLabel: plan.YLabel, Colors: plan.Colors},
				Mermaid: mermaid,
			}

			mu.Lock()
			doneCount++
			sendProgress(pw, "report_progress", map[string]any{
				"phase": "executor", "current": doneCount, "total": len(plans), "title": plan.Title,
			})
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// --- Phase 3: Summarizer ---

func (t *GenerateReport) runSummarizer(ctx context.Context, prompt string, charts []ReportChart) (string, error) {
	var sb strings.Builder
	sb.WriteString("用户需求: ")
	sb.WriteString(prompt)
	sb.WriteString("\n\n")
	for i, c := range charts {
		fmt.Fprintf(&sb, "## 图表 %d: %s\n\n", i+1, c.Spec.Title)
		sb.WriteString(c.Mermaid)
		sb.WriteString("\n\n")
	}

	resp, err := t.ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: summarizerSystemPrompt},
			{Role: openai.ChatMessageRoleUser, Content: sb.String()},
		},
	})
	if err != nil {
		return "", fmt.Errorf("汇总阶段 AI 调用失败: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("AI 返回为空")
	}
	return resp.Choices[0].Message.Content, nil
}

// --- Phase 4: Converter ---

func (t *GenerateReport) runConverter(ctx context.Context, markdown, format string, pw agent.ProgressWriter) (string, error) {
	title := extractTitle(markdown)
	var filePath string
	var err error
	switch format {
	case "pdf":
		filePath, err = t.converter.ConvertPDF(markdown, title)
	case "docx":
		filePath, err = t.converter.ConvertDOCX(markdown, title)
	default:
		return "", fmt.Errorf("不支持的格式: %s", format)
	}
	if err != nil {
		return "", err
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开转换文件失败: %w", err)
	}
	defer f.Close()

	storagePath := "reports/" + filepath.Base(filePath)
	if err := t.fileStorage.Save(ctx, storagePath, f); err != nil {
		return "", fmt.Errorf("保存文件到存储失败: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return "", err
	}

	fi, _ := os.Stat(filePath)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	fileRecord := &model.File{
		Filename:    filepath.Base(filePath),
		MimeType:    mimeTypeByFormat(format),
		Size:        size,
		StoragePath: filepath.ToSlash(storagePath),
		UploadedBy:  0,
	}
	if err := t.fileRepo.Create(fileRecord); err != nil {
		return "", fmt.Errorf("创建文件记录失败: %w", err)
	}
	return fmt.Sprintf("/api/v1/files/%d/download", fileRecord.ID), nil
}

// --- helpers ---

func sendProgress(pw agent.ProgressWriter, typ string, data map[string]any) {
	if pw != nil {
		pw(typ, data)
	}
}

// chatAndParse 本地 AI 调用辅助函数（避免对 service 包的硬依赖）。
func chatAndParse[T any](ai *aiclient.Client, ctx context.Context, system, user, parseErrMsg string) (*T, error) {
	resp, err := ai.ChatCompletion(ctx, openai.ChatCompletionRequest{
		Messages: []openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleSystem, Content: system},
			{Role: openai.ChatMessageRoleUser, Content: user},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("AI服务暂不可用")
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("AI返回为空")
	}
	text := resp.Choices[0].Message.Content
	// 与 service.ChatAndParse 一致的清理逻辑：先搜代码围栏再兜底取 JSON 边界
	for _, prefix := range []string{"```json", "```"} {
		if idx := strings.Index(text, prefix); idx >= 0 {
			text = text[idx+len(prefix):]
			break
		}
	}
	if idx := strings.LastIndex(text, "```"); idx >= 0 {
		text = text[:idx]
	}
	// 兜底：取最外层的 JSON 边界
	if trimmed := strings.TrimLeft(text, " \t\r\n"); len(trimmed) > 0 && trimmed[0] == '[' {
		if end := strings.LastIndexByte(text, ']'); end >= 0 {
			text = text[:end+1]
		}
	} else if start := strings.IndexByte(text, '{'); start >= 0 {
		if end := strings.LastIndexByte(text, '}'); end >= start {
			text = text[start : end+1]
		}
	}
	text = strings.TrimSpace(text)

	var result T
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return nil, errors.New(parseErrMsg)
	}
	return &result, nil
}

func extractTitle(md string) string {
	for _, line := range strings.Split(md, "\n") {
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

func mimeTypeByFormat(format string) string {
	switch format {
	case "pdf":
		return "application/pdf"
	case "docx":
		return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
	default:
		return "application/octet-stream"
	}
}
