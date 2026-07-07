package builtin

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type ReportConverter struct {
	baseURL    string
	httpClient *http.Client
}

func NewReportConverter(addr string, timeoutSec int) *ReportConverter {
	return &ReportConverter{
		baseURL: fmt.Sprintf("http://%s", addr),
		httpClient: &http.Client{Timeout: time.Duration(timeoutSec) * time.Second},
	}
}

type convertRequest struct {
	Markdown string `json:"markdown"`
	Title    string `json:"title"`
}

type convertResponse struct {
	FilePath string `json:"file_path"`
}

func (c *ReportConverter) ConvertPDF(markdown, title string) (string, error) {
	return c.convert("/convert/pdf", markdown, title)
}

func (c *ReportConverter) ConvertDOCX(markdown, title string) (string, error) {
	return c.convert("/convert/docx", markdown, title)
}

func (c *ReportConverter) convert(endpoint, markdown, title string) (string, error) {
	body, _ := json.Marshal(convertRequest{Markdown: markdown, Title: title})
	resp, err := c.httpClient.Post(c.baseURL+endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("converter request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("converter returned %d: %s", resp.StatusCode, string(raw))
	}
	var cr convertResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", fmt.Errorf("converter parse failed: %w", err)
	}
	if _, err := os.Stat(cr.FilePath); err != nil {
		return "", fmt.Errorf("converter file not found: %s", cr.FilePath)
	}
	return cr.FilePath, nil
}
