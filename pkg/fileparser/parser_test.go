package fileparser

import (
	"bytes"
	"strings"
	"testing"
)

func TestStripXMLTags(t *testing.T) {
	input := "<w:p><w:r><w:t>Hello World</w:t></w:r></w:p>"
	want := "Hello World"
	got := stripXMLTags(input)
	if got != want {
		t.Errorf("stripXMLTags(%q) = %q, want %q", input, got, want)
	}
}

func TestParseDOCX(t *testing.T) {
	// 创建一个最小 DOCX（空 ZIP 含 word/document.xml）
	// 无实际文档时跳过测试
	t.Skip("requires a real .docx file")
}

func TestParse_UnsupportedExt(t *testing.T) {
	_, err := Parse(bytes.NewReader(nil), 0, ".txt")
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected unsupported error, got %v", err)
	}
}
