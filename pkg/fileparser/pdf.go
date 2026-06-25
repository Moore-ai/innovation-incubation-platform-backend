package fileparser

import (
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/ledongthuc/pdf"
)

func parsePDF(r io.ReaderAt, size int64) (string, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var buf strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		text, err := page.GetPlainText(nil)
		if err != nil {
			slog.Warn("parse pdf page failed", "page", i, "error", err)
			continue
		}
		buf.WriteString(text)
		buf.WriteByte('\n')
	}
	return buf.String(), nil
}
