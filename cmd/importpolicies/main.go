package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/database"
	"innovation-incubation-platform-backend/internal/model"
)

type legalBasisRow struct {
	title  string
	clause string
	fileID uint
}

func main() {
	detailsPath := flag.String("details", firstExistingPath("zcdx_details_with_policy_links.csv", "policy-samples/zcdx_details_with_policy_links.csv"), "政策详情 CSV")
	basisPath := flag.String("basis", firstExistingPath("zcdx_policy_basis_links.csv", "policy-samples/zcdx_policy_basis_links.csv"), "政策依据 CSV")
	from := flag.Int("from", 0, "起始行，从 0 开始")
	limit := flag.Int("limit", 0, "导入条数，0 表示全部")
	flag.Parse()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.MustLoad("config/config.yaml")
	db := database.MustInit(cfg)

	basisMap := readBasisMap(*basisPath)
	rows, col := readCSV(*detailsPath)
	if *from >= len(rows) {
		slog.Error("from 超出总行数", "from", *from, "total", len(rows))
		os.Exit(1)
	}
	rows = rows[*from:]
	if *limit > 0 && *limit < len(rows) {
		rows = rows[:*limit]
	}

	now := time.Now()
	created, updated, failed := 0, 0, 0
	for idx, row := range rows {
		serviceID := field(row, col, "id")
		title := field(row, col, "serviceName")
		dept := field(row, col, "orgName")
		area := field(row, col, "areaName")
		condition := field(row, col, "applyCondition")
		standard := field(row, col, "cashStandard")
		startDate := field(row, col, "applyStartTime")
		endDate := field(row, col, "applyEndTime")
		if title == "" {
			failed++
			slog.Warn("跳过无标题行", "row", *from+idx+2)
			continue
		}

		req := &model.PolicyRequirement{}
		if condition != "" {
			req.ApplicationCondition = &condition
		}
		if standard != "" {
			req.FulfillmentCriteria = &standard
		}
		for _, basis := range basisMap[serviceID] {
			req.LegalBasis = append(req.LegalBasis, model.LegalBasisFile{
				Title:          basis.title,
				SpecificClause: basis.clause,
				FileID:         basis.fileID,
			})
		}
		extracted := fallbackExtractedPolicy(title, condition, standard, area, req)

		var existing model.Policy
		err := db.Where("title = ? AND department = ?", title, dept).First(&existing).Error
		if err == nil {
			updates := map[string]any{
				"target_role":      model.TargetRoleEnterprise,
				"requirements":     req,
				"start_date":       startDate,
				"end_date":         endDate,
				"status":           model.PolicyPublished,
				"published_at":     firstTime(existing.PublishedAt, &now),
				"extracted_fields": extracted,
			}
			if err := db.Model(&existing).Updates(updates).Error; err != nil {
				failed++
				slog.Error("更新政策失败", "title", title, "error", err)
				continue
			}
			updated++
			continue
		}

		policy := &model.Policy{
			TargetRole:      model.TargetRoleEnterprise,
			Title:           title,
			Department:      dept,
			Requirements:    req,
			StartDate:       startDate,
			EndDate:         endDate,
			Status:          model.PolicyPublished,
			PublishedAt:     &now,
			ExtractedFields: extracted,
			ChangeLog:       []string{now.Format("2006-01-02 15:04:05")},
		}
		if err := db.Create(policy).Error; err != nil {
			failed++
			slog.Error("创建政策失败", "title", title, "error", err)
			continue
		}
		created++
	}

	fmt.Printf("政策导入完成：新增 %d，更新 %d，失败 %d\n", created, updated, failed)
}

func firstTime(existing *time.Time, fallback *time.Time) *time.Time {
	if existing != nil {
		return existing
	}
	return fallback
}

func firstExistingPath(paths ...string) string {
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	if len(paths) == 0 {
		return ""
	}
	return paths[0]
}

func readCSV(path string) ([][]string, map[string]int) {
	file, err := os.Open(path)
	if err != nil {
		slog.Error("打开 CSV 失败", "path", path, "error", err)
		os.Exit(1)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.LazyQuotes = true
	records, err := reader.ReadAll()
	if err != nil {
		slog.Error("读取 CSV 失败", "path", path, "error", err)
		os.Exit(1)
	}
	if len(records) < 2 {
		slog.Error("CSV 没有数据", "path", path)
		os.Exit(1)
	}
	col := make(map[string]int)
	for i, header := range records[0] {
		col[cleanHeader(header)] = i
	}
	return records[1:], col
}

func readBasisMap(path string) map[string][]legalBasisRow {
	result := map[string][]legalBasisRow{}
	rows, col := readCSV(path)
	for _, row := range rows {
		serviceID := field(row, col, "serviceId")
		if serviceID == "" {
			continue
		}
		basis := legalBasisRow{
			title:  field(row, col, "policyTitle"),
			clause: field(row, col, "clause"),
		}
		if value := field(row, col, "fileID"); value != "" {
			if id, err := strconv.ParseUint(value, 10, 64); err == nil {
				basis.fileID = uint(id)
			}
		}
		result[serviceID] = append(result[serviceID], basis)
	}
	return result
}

func cleanHeader(value string) string {
	return strings.TrimSpace(strings.TrimPrefix(value, "\ufeff"))
}

func field(row []string, col map[string]int, name string) string {
	index, ok := col[name]
	if !ok || index >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[index])
}

func fallbackExtractedPolicy(title, condition, standard, area string, req *model.PolicyRequirement) *model.ExtractedPolicy {
	documents := []string{}
	for _, basis := range req.LegalBasis {
		if basis.Title != "" {
			documents = append(documents, basis.Title)
		}
	}
	return &model.ExtractedPolicy{
		PolicyName:        title,
		PolicySummary:     strings.Join(nonEmpty(condition, standard), "；"),
		ApplicableRegion:  area,
		SubsidyType:       "奖补",
		Subsidies:         []model.SubsidyDetail{{Condition: condition, Amount: standard}},
		RequiredDocuments: documents,
	}
}

func nonEmpty(values ...string) []string {
	result := []string{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
