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

func TestMatchExact(t *testing.T) {
	files := []model.File{
		{BaseModel: model.BaseModel{ID: 1}, Filename: "营业执照.pdf"},
	}
	r := Match("营业执照", []string{"pdf"}, files, testCfg)
	if r == nil || r.FileID != 1 {
		t.Errorf("expected match file_id=1, got %v", r)
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
