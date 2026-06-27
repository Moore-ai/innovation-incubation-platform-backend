package fileparser

import (
	"fmt"
	"io"
	"log/slog"
	"strings"
)

var supportedExts = map[string]bool{
	".pdf": true, ".docx": true, ".doc": true,
	".xlsx": true, ".xls": true, ".pptx": true, ".ppt": true,
	".html": true, ".htm": true, ".txt": true, ".csv": true,
	".md": true, ".xml": true, ".json": true, ".zip": true,
	".epub": true,
}

func Parse(r io.Reader, size int64, ext string) (string, error) {
	ext = strings.ToLower(ext)

	// 优先 sidecar（先校验扩展名白名单）
	if supportedExts[ext] {
		md, err := parseViaSidecar(r, ext)
		if err == nil {
			return md, nil
		}
		slog.Warn("sidecar parse failed, falling back to local", "ext", ext, "error", err)
	}

	// 降级：仅 .pdf / .docx
	ra, ok := r.(io.ReaderAt)
	if !ok {
		return "", fmt.Errorf("local parser requires io.ReaderAt")
	}
	switch ext {
	case ".docx":
		return parseDOCX(ra, size)
	case ".pdf":
		return parsePDF(ra, size)
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}
