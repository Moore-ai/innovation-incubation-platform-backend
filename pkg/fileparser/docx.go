package fileparser

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
)

func parseDOCX(r io.ReaderAt, size int64) (string, error) {
	zr, err := zip.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("open docx zip: %w", err)
	}
	var documentFile *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			documentFile = f
			break
		}
	}
	if documentFile == nil {
		return "", fmt.Errorf("word/document.xml not found in docx")
	}
	rc, err := documentFile.Open()
	if err != nil {
		return "", fmt.Errorf("open word/document.xml: %w", err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read word/document.xml: %w", err)
	}

	return stripXMLTags(string(raw)), nil
}

func stripXMLTags(s string) string {
	var buf bytes.Buffer
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			buf.WriteRune(r)
		}
	}
	// 清理多余空白
	result := strings.TrimSpace(buf.String())
	return result
}
