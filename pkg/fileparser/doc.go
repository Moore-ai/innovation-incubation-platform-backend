package fileparser

import (
	"bytes"
	"fmt"
	"io"
	"os/exec"
)

func parseDOC(r io.Reader) (string, error) {
	if !pandocAvailable {
		return "", fmt.Errorf("pandoc not available")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read doc input: %w", err)
	}

	cmd := exec.Command("pandoc", "-f", "doc", "-t", "plain", "--wrap=none")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pandoc failed: %w", err)
	}
	return string(out), nil
}
