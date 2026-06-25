package fileparser

import (
	"fmt"
	"io"

	"github.com/ledongthuc/pdf"
)

func parsePDF(r io.ReaderAt, size int64) (string, error) {
	reader, err := pdf.NewReader(r, size)
	if err != nil {
		return "", fmt.Errorf("open pdf: %w", err)
	}
	var buf string
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		buf += text + "\n"
	}
	return buf, nil
}
