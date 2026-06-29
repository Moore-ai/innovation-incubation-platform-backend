package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/internal/repository"
	"innovation-incubation-platform-backend/internal/service"
	"innovation-incubation-platform-backend/pkg/aiclient"
)

func main() {
	from := flag.Int("from", 0, "起始行（跳过前 N 条）")
	limit := flag.Int("limit", 10, "本次导入条数")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.MustLoad("config/config.yaml")
	db := database.MustInit(cfg)

	aiClient := aiclient.New(cfg.AI.OpenAI.BaseURL, cfg.AI.OpenAI.APIKey, cfg.AI.OpenAI.Model, cfg.AI.OpenAI.TimeoutSeconds)
	fileRepo := repository.NewFileRepo(db)
	aiSvc := service.NewAIService(aiClient, nil, nil, fileRepo, cfg)

	var embedClient *aiclient.EmbeddingClient
	if cfg.AI.Embedding.APIKey != "" {
		embedClient = aiclient.NewEmbeddingClient(cfg.AI.Embedding)
	} else {
		slog.Warn("embedding 未配置，跳过向量生成")
	}

	f, err := os.Open("policy-samples/zcdx_details_with_policy_links.csv")
	if err != nil {
		slog.Error("打开 CSV 失败", "error", err)
		os.Exit(1)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		slog.Error("读取 CSV 失败", "error", err)
		os.Exit(1)
	}
	if len(records) < 2 {
		slog.Error("CSV 数据不足（无内容行）")
		os.Exit(1)
	}

	header := records[0]
	col := make(map[string]int)
	for i, h := range header {
		col[h] = i
	}
	for _, c := range []string{"serviceName", "orgName", "applyCondition", "cashStandard"} {
		if _, ok := col[c]; !ok {
			slog.Error("CSV 缺少必要列", "column", c)
			os.Exit(1)
		}
	}

	rows := records[1:]
	if *from >= len(rows) {
		slog.Error("from 超出总行数", "from", *from, "total", len(rows))
		os.Exit(1)
	}
	rows = rows[*from:]
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
	}

	slog.Info("开始导入", "条数", len(rows), "起始行", *from)

	ctx := context.Background()
	now := time.Now()
	success, apiCalls := 0, 0

	for i, row := range rows {
		title := row[col["serviceName"]]
		dept := row[col["orgName"]]
		condition := row[col["applyCondition"]]
		standard := row[col["cashStandard"]]
		startDate := csvField(row, col, "applyStartTime")
		endDate := csvField(row, col, "applyEndTime")

		slog.Info("导入", "序号", *from+i+2, "标题", title)

		req := &model.PolicyRequirement{}
		if condition != "" {
			req.ApplicationCondition = &condition
		}
		if standard != "" {
			req.FulfillmentCriteria = &standard
		}

		policy := &model.Policy{
			TargetRole:   model.RoleEnterprise,
			Title:        title,
			Department:   dept,
			Requirements: req,
			StartDate:    startDate,
			EndDate:      endDate,
			Status:       model.PolicyPublished,
			PublishedAt:  &now,
			ChangeLog:    []string{now.Format("2006-01-02 15:04:05")},
		}

		// 第一步：保存基础字段
		if err := db.Create(policy).Error; err != nil {
			slog.Error("基础入库失败", "标题", title, "error", err)
			continue
		}
		slog.Info("基础入库成功", "ID", policy.ID, "标题", title)

		// 第二步：AI 提取（ExtractPolicy 内部已内置重试，无需外部包装）
		slog.Info("AI 提取结构化字段", "标题", title)
		if err := aiSvc.ExtractPolicy(ctx, policy); err != nil {
			slog.Error("AI 提取失败，跳过", "标题", title, "error", err)
			continue
		}
		apiCalls++

		// 第三步：生成向量
		if embedClient != nil {
			slog.Info("生成向量", "标题", title)
			text := buildEmbedText(policy, condition, standard)
			emb, err := embedClient.Embed(ctx, text)
			if err != nil {
				slog.Warn("向量生成失败，跳过向量", "标题", title, "error", err)
			} else {
				policy.Embedding = emb
			}
			apiCalls++
		}

		// 第四步：更新结构化字段和向量
		updates := map[string]any{}
		if policy.ExtractedFields != nil {
			updates["extracted_fields"] = policy.ExtractedFields
		}
		if policy.Embedding != nil {
			updates["embedding"] = policy.Embedding
		}
		if len(updates) > 0 {
			if err := db.Model(policy).Updates(updates).Error; err != nil {
				slog.Error("更新结构化字段失败", "标题", title, "error", err)
			} else {
				slog.Info("结构化字段更新成功", "标题", title)
			}
		}

		success++
		slog.Info("导入成功", "序号", *from+i+2, "标题", title)
	}

	slog.Info("导入完成", "成功", success, "失败", len(rows)-success, "API调用", apiCalls)
	fmt.Printf("\n导入结果：%d/%d 成功，共 %d 次 API 调用\n", success, len(rows), apiCalls)
}

func csvField(row []string, col map[string]int, name string) string {
	if i, ok := col[name]; ok && i < len(row) {
		return strings.TrimSpace(row[i])
	}
	return ""
}

func buildEmbedText(p *model.Policy, condition, standard string) string {
	var parts []string
	parts = append(parts, p.Title)
	if p.ExtractedFields != nil {
		if p.ExtractedFields.PolicySummary != "" {
			parts = append(parts, p.ExtractedFields.PolicySummary)
		}
		for _, s := range p.ExtractedFields.Subsidies {
			parts = append(parts, "补贴："+s.Condition+"，"+s.Amount)
		}
	}
	if condition != "" {
		parts = append(parts, condition)
	}
	if standard != "" {
		parts = append(parts, standard)
	}
	return strings.Join(parts, "。")
}
