package builtin

import (
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
	"innovation-incubation-platform-backend/pkg/database"

	"gorm.io/gorm"
)

var testConfig = &TableConfig{
	Table:    "enterprises",
	Columns: []ColumnDef{
		{Name: "name", Type: "string", FilterMode: "like"},
		{Name: "credit_code", Type: "string", FilterMode: "exact"},
		{Name: "industry", Type: "string", FilterMode: "exact"},
		{Name: "scale", Type: "string", FilterMode: "exact"},
		{Name: "address", Type: "string", FilterMode: "like"},
		{Name: "created_at", Type: "string", FilterMode: "range"},
	},
	Aggregates: []string{"count"},
}

// testSeedPrefix 种子数据唯一前缀，用于隔离真实数据
const testSeedPrefix = "ZTEST_"

func setupQueryTest(t *testing.T) (*gorm.DB, *QueryEngine) {
	t.Helper()
	db := openTestDB(t)
	db.AutoMigrate(&model.Enterprise{})
	engine := &QueryEngine{}
	t.Cleanup(func() {
		db.Exec("DELETE FROM enterprises WHERE user_id >= 99000")
	})
	return db, engine
}

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	loadDotEnvForTest()
	port, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	if port == 0 {
		port = 5432
	}
	cfg := config.DBConfig{
		Host:     os.Getenv("DB_HOST"),
		Port:     port,
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		Name:     os.Getenv("DB_NAME"),
		SSLMode:  "disable",
		LogLevel: "warn",
	}
	db, err := database.NewDB(cfg)
	if err != nil {
		t.Skipf("跳过集成测试：%v", err)
	}
	return db
}

func findProjectRoot() string {
	dir, _ := os.Getwd()
	sep := string(os.PathSeparator)
	for {
		if _, err := os.Stat(dir + sep + "go.mod"); err == nil {
			return dir
		}
		idx := strings.LastIndex(dir, sep)
		if idx <= 0 {
			break
		}
		dir = dir[:idx]
	}
	wd, _ := os.Getwd()
	return wd
}

func loadDotEnvForTest() {
	data, err := os.ReadFile(findProjectRoot() + "/.env")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if idx := strings.IndexByte(line, '='); idx > 0 {
			key := strings.TrimSpace(line[:idx])
			val := strings.TrimSpace(line[idx+1:])
			if os.Getenv(key) == "" {
				os.Setenv(key, val)
			}
		}
	}
}

func seedEnterprises(t *testing.T, db *gorm.DB) {
	t.Helper()
	// 先清再种，避免与旧残留数据信用代码冲突
	db.Exec("DELETE FROM enterprises WHERE credit_code LIKE 'ZTEST_C%'")
	now := time.Now()
	ents := []model.Enterprise{
		{BaseModel: model.BaseModel{CreatedAt: now}, UserID: 99001, Name: "ZTEST_科技公司", Industry: "信息技术", Scale: "中型", Address: "合肥市高新区", CreditCode: "ZTEST_C01"},
		{BaseModel: model.BaseModel{CreatedAt: now}, UserID: 99002, Name: "ZTEST_生物公司", Industry: "生物医药", Scale: "小型", Address: "合肥市经开区", CreditCode: "ZTEST_C02"},
		{BaseModel: model.BaseModel{CreatedAt: now}, UserID: 99003, Name: "ZTEST_制造公司", Industry: "智能制造", Scale: "大型", Address: "合肥市高新区", CreditCode: "ZTEST_C03"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -1, 0)}, UserID: 99004, Name: "ZTEST_上月入驻公司", Industry: "信息技术", Scale: "中型", Address: "北京市海淀区", CreditCode: "ZTEST_C04"},
		{BaseModel: model.BaseModel{CreatedAt: now.AddDate(0, -2, 0)}, UserID: 99005, Name: "ZTEST_两月前入驻公司", Industry: "新能源", Scale: "大型", Address: "上海市浦东", CreditCode: "ZTEST_C05"},
	}
	if err := db.Create(&ents).Error; err != nil {
		t.Fatalf("seed failed: %v", err)
	}
}

// filterSeed 返回仅筛选种子数据的参数。
func filterSeed() map[string]any {
	// 种子企业名称都含"ZTEST"，以此隔离真实数据
	return map[string]any{"name": "ZTEST_"}
}

func TestQuery_AllRows(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, filterSeed())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 5 {
		t.Errorf("expected 5 seed rows, got %d", result.RowCount)
	}
}

func TestQuery_ExactFilter(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":        testSeedPrefix,
		"credit_code": "ZTEST_C01",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 1 {
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}

func TestQuery_LikeFilter(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	// 只用种子数据的名称匹配
	result, err := engine.Query(db, testConfig, map[string]any{"name": "ZTEST_科技"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 1 { // ZTEST_科技公司 only
		t.Errorf("expected 1 row, got %d", result.RowCount)
	}
}

func TestQuery_RangeFilter(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	params := filterSeed()
	params["created_at_from"] = time.Now().AddDate(0, 0, -1).Format("2006-01-02")
	params["created_at_to"] = time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	result, err := engine.Query(db, testConfig, params)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 3 { // today's 3 seeds
		t.Errorf("expected 3 rows, got %d", result.RowCount)
	}
}

func TestQuery_GroupByCount(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":      testSeedPrefix,
		"group_by":  "industry",
		"aggregate": "count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Columns) != 2 || result.Columns[0] != "industry" || result.Columns[1] != "count" {
		t.Errorf("unexpected columns: %v", result.Columns)
	}
	if result.RowCount < 1 {
		t.Error("expected at least 1 group row")
	}
}

func TestQuery_GroupByPeriod(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":             testSeedPrefix,
		"group_by_period":  "month",
		"aggregate":        "count",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Columns) != 2 {
		t.Errorf("expected 2 columns (date_trunc, count), got %v", result.Columns)
	}
}

func TestQuery_OrderByDesc(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":     testSeedPrefix,
		"order_by": "-id",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 5 {
		t.Error("expected 5 seed rows")
	}
}

func TestQuery_Limit(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":  testSeedPrefix,
		"limit": float64(2),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows (limit), got %d", result.RowCount)
	}
}

func TestQuery_CombinedFilters(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{
		"name":     testSeedPrefix,
		"industry": "信息技术",
		"scale":    "中型",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 2 {
		t.Errorf("expected 2 rows, got %d", result.RowCount)
	}
}

func TestQuery_NoMatch(t *testing.T) {
	db, engine := setupQueryTest(t)
	seedEnterprises(t, db)

	result, err := engine.Query(db, testConfig, map[string]any{"credit_code": "NONEXISTENT"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RowCount != 0 {
		t.Errorf("expected 0 rows, got %d", result.RowCount)
	}
}
