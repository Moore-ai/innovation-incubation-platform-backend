package fileparser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

var (
	globalClient    *http.Client
	sidecarBaseURL string // 如 "http://127.0.0.1:54321"
)

type convertResponse struct {
	Markdown string `json:"markdown"`
	Error    string `json:"error"`
}

func parseViaSidecar(r io.Reader, ext string) (string, error) {
	if globalClient == nil {
		return "", fmt.Errorf("sidecar not available")
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	filePart, _ := w.CreateFormFile("file", "file"+ext)
	io.Copy(filePart, r)
	w.WriteField("ext", ext)
	w.Close()

	resp, err := globalClient.Post(sidecarBaseURL+"/convert", w.FormDataContentType(), body)
	if err != nil {
		return "", fmt.Errorf("sidecar call failed: %w", err)
	}
	defer resp.Body.Close()

	var cr convertResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("sidecar decode failed: %w", err)
	}
	if cr.Error != "" {
		return "", fmt.Errorf("sidecar error: %s", cr.Error)
	}
	return cr.Markdown, nil
}
