package fileparser

import (
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strings"
)

var pandocAvailable bool

func init() {
	_, err := exec.LookPath("pandoc")
	if err != nil {
		slog.Warn("pandoc not found in PATH, .doc files will not be parsed")
		pandocAvailable = false
	} else {
		pandocAvailable = true
	}
}

func Parse(r io.Reader, size int64, ext string) (string, error) {
	switch strings.ToLower(ext) {
	case ".docx":
		ra, ok := r.(io.ReaderAt)
		if !ok {
			return "", fmt.Errorf("docx parser requires io.ReaderAt")
		}
		return parseDOCX(ra, size)
	case ".pdf":
		ra, ok := r.(io.ReaderAt)
		if !ok {
			return "", fmt.Errorf("pdf parser requires io.ReaderAt")
		}
		return parsePDF(ra, size)
	case ".doc":
		return parseDOC(r)
	default:
		return "", fmt.Errorf("unsupported file extension: %s", ext)
	}
}
