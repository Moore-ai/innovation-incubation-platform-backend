package builtin

import "gorm.io/gorm"

// TableConfig 定义一张表的查询配置。
type TableConfig struct {
	Table       string              // 表名
	Description string              // 工具 Description
	Columns     []ColumnDef         // 可筛选/分组字段
	TimeColumn  string              // group_by_period 使用的日期列，默认 "created_at"
	Aggregates  []string            // 支持的聚合方式，默认 ["count"]
}

// ColumnDef 定义列。
type ColumnDef struct {
	Name       string   // 列名
	Type       string   // string / int / bool / float
	FilterMode string   // exact / like / range
	EnumValues []string // 枚举值（可选，填入 InputSchema enum）
}

// QueryResult 统一返回格式。
type QueryResult struct {
	Columns  []string `json:"columns"`
	Rows     [][]any  `json:"rows"`
	RowCount int      `json:"row_count"`
}

// QueryEngine 参数化查询后端。
type QueryEngine struct{}

func (e *QueryEngine) Query(db *gorm.DB, cfg *TableConfig, params map[string]any) (*QueryResult, error) {
	timeCol := cfg.TimeColumn
	if timeCol == "" {
		timeCol = "created_at"
	}

	q := db.Table(cfg.Table)

	// 应用等值过滤
	for _, col := range cfg.Columns {
		if col.FilterMode == "range" {
			if from, ok := params[col.Name+"_from"].(string); ok && from != "" {
				q = q.Where(col.Name+" >= ?", from)
			}
			if to, ok := params[col.Name+"_to"].(string); ok && to != "" {
				q = q.Where(col.Name+" <= ?", to)
			}
			continue
		}
		v, ok := params[col.Name]
		if !ok {
			continue
		}
		switch col.FilterMode {
		case "like":
			if s, ok := v.(string); ok && s != "" {
				q = q.Where(col.Name+" ILIKE ?", "%"+s+"%")
			}
		case "exact", "":
			q = q.Where(col.Name+" = ?", v)
		}
	}

	// group_by + aggregate
	gby, _ := params["group_by"].(string)
	period, _ := params["group_by_period"].(string)
	agg, _ := params["aggregate"].(string)
	aggField := "score" // 仅 performance_submissions 需要用

	var selectCols []string
	if gby != "" {
		selectCols = append(selectCols, gby)
		q = q.Group(gby)
	}
	if period != "" {
		periodExpr := "DATE_TRUNC('" + period + "', " + timeCol + ")"
		selectCols = append(selectCols, periodExpr)
		q = q.Group(periodExpr)
	}
	switch agg {
	case "count", "":
		selectCols = append(selectCols, "COUNT(*) AS count")
	case "avg":
		selectCols = append(selectCols, "AVG("+aggField+") AS avg")
	case "max":
		selectCols = append(selectCols, "MAX("+aggField+") AS max")
	case "min":
		selectCols = append(selectCols, "MIN("+aggField+") AS min")
	}
	if len(selectCols) > 0 {
		q = q.Select(selectCols)
	}

	// order / limit
	limit := 100
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	if limit > 2000 {
		limit = 2000
	}
	q = q.Limit(limit)

	orderBy, _ := params["order_by"].(string)
	if orderBy != "" {
		dir := "ASC"
		if orderBy[0] == '-' {
			dir = "DESC"
			orderBy = orderBy[1:]
		}
		q = q.Order(orderBy + " " + dir)
	}

	// 执行
	var rows []map[string]any
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}

	columns := extractColumns(selectCols, gby, period, agg)
	result := &QueryResult{Columns: columns, Rows: make([][]any, len(rows)), RowCount: len(rows)}
	for i, row := range rows {
		for _, c := range columns {
			result.Rows[i] = append(result.Rows[i], row[c])
		}
	}
	return result, nil
}

func extractColumns(selectCols []string, gby, period, agg string) []string {
	var cols []string
	if gby != "" {
		cols = append(cols, gby)
	}
	if period != "" {
		cols = append(cols, period)
	}
	switch agg {
	case "count", "":
		cols = append(cols, "count")
	default:
		cols = append(cols, agg)
	}
	return cols
}
