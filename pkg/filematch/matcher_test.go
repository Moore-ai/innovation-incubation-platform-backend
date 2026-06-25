package filematch

import (
	"testing"

	"innovation-incubation-platform-backend/config"
	"innovation-incubation-platform-backend/internal/model"
)

var testCfg = config.FileMatchConfig{
	WeightJaro:    0.4,
	WeightKeyword: 0.4,
	WeightPrefix:  0.2,
	Threshold:     0.6,
}

func TestJaroWinkler(t *testing.T) {
	s := jaroWinkler("营业执照复印件", "营业执照")
	if s <= 0 {
		t.Errorf("expected positive similarity, got %f", s)
	}
}

func TestJaroWinklerIdentical(t *testing.T) {
	s := jaroWinkler("营业执照", "营业执照")
	if s != 1.0 {
		t.Errorf("identical strings should get score 1.0, got %f", s)
	}
}

func TestJaroWinklerEmpty(t *testing.T) {
	s := jaroWinkler("", "")
	if s != 1.0 {
		t.Errorf("both empty should return 1.0, got %f", s)
	}
}

func TestMatchExact(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 1}, Filename: "营业执照.pdf"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r == nil || r.FileID != 1 {
		t.Errorf("expected match file_id=1, got %v", r)
	}
	if r.Warning != "" {
		t.Errorf("expected no warning, got %s", r.Warning)
	}
}

func TestMatchExtensionWarning(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 2}, Filename: "营业执照.jpg"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r == nil {
		t.Fatal("expected match")
	}
	if r.Warning == "" {
		t.Error("expected extension warning")
	}
}

func TestMatchBelowThreshold(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 3}, Filename: "年终总结报告.docx"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r != nil {
		t.Errorf("expected no match, got file_id=%d score=%.2f", r.FileID, r.Score)
	}
}

func TestMatchFuzzy(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 4}, Filename: "营业执照副本.pdf"},
		{BaseModel: model.BaseModel{ID: 5}, Filename: "企业法人营业执照扫描件.jpg"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r == nil || r.FileID != 4 {
		t.Errorf("expected best match file_id=4 (PDF), got %v", r)
	}
}

func TestMatchStopWords(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 6}, Filename: "营业执照复印件.png"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r == nil {
		t.Fatal("expected match despite stop word and extension mismatch")
	}
	if r.Warning == "" {
		t.Error("expected extension warning for .png vs pdf")
	}
}

func TestMatchNoFormats(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 7}, Filename: "营业执照.bmp"},
	}
	r := Match("营业执照", nil, files, testCfg)
	if r == nil || r.FileID != 7 {
		t.Errorf("expected match with no format restriction, got %v", r)
	}
	if r.Warning != "" {
		t.Errorf("expected no warning when formats is empty, got %s", r.Warning)
	}
}

func TestSearchReturnsSorted(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 8}, Filename: "营业执照副本.jpg"},
		{BaseModel: model.BaseModel{ID: 9}, Filename: "公司章程.doc"},
		{BaseModel: model.BaseModel{ID: 10}, Filename: "企业营业执照.pdf"},
	}
	results := Search("营业执照", nil, files, testCfg)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Score < results[1].Score {
		t.Error("results should be sorted descending by score")
	}
}

func TestExtractKeywords(t *testing.T) {
	kw := tokenize("营业执照复印件扫描件")
	for _, w := range kw {
		if w == "复印件" || w == "扫描件" {
			t.Errorf("stop word '%s' should be removed", w)
		}
	}
}
