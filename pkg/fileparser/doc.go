package fileparser

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"
)

func parseDOC(r io.Reader) (string, error) {
	if !pandocAvailable {
		return "", fmt.Errorf("pandoc not available")
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read doc input: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "pandoc", "-f", "doc", "-t", "plain", "--wrap=none")
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("pandoc failed: %w", err)
	}
	return string(out), nil
}
