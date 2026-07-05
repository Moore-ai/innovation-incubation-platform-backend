// Package mcpchart 提供 Python matplotlib 图表生成的 MCP sidecar 客户端。
package mcpchart

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// ChartRequest 发给 Python 进程的 JSON-RPC 请求。
type ChartRequest struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params"`
}

// ChartResult 渲染结果。
type ChartResult struct {
	FileName string `json:"file_name"`
	FilePath string `json:"file_path"`
	Size     int    `json:"size"`
}

// Client 管理 Python 图表渲染子进程。
type Client struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
}

// findProjectRoot 向上查找包含 go.mod 的目录。
func findProjectRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return dir
		}
		dir = parent
	}
}

// NewClient 启动 Python render.py 子进程，使用指定的 venv。
func NewClient(venvPath string) (*Client, error) {
	python := filepath.Join(venvPath, "Scripts", "python.exe") // Windows
	if _, err := os.Stat(python); err != nil {
		python = filepath.Join(venvPath, "bin", "python") // Linux/Mac
	}
	if _, err := os.Stat(python); err != nil {
		return nil, fmt.Errorf("venv python not found: %s", venvPath)
	}

	root := findProjectRoot()
	scriptPath := filepath.Join(root, "pkg", "mcpchart", "render.py")
	cmd := exec.Command(python, scriptPath)
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start python: %w", err)
	}

	return &Client{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewScanner(stdout),
	}, nil
}

// Render 渲染图表，返回结果。
func (c *Client) Render(params map[string]any) (*ChartResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	req := ChartRequest{Method: "render_chart", Params: params}
	data, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	if _, err := fmt.Fprintln(c.stdin, string(data)); err != nil {
		return nil, fmt.Errorf("write to python: %w", err)
	}

	if !c.stdout.Scan() {
		return nil, fmt.Errorf("python stdout closed: %w", c.stdout.Err())
	}

	var resp struct {
		Result ChartResult `json:"result"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(c.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w (raw: %s)", err, c.stdout.Text())
	}
	if resp.Error != "" {
		return nil, fmt.Errorf("python error: %s", resp.Error)
	}
	return &resp.Result, nil
}

// Close 终止子进程。
func (c *Client) Close() {
	if c.stdin != nil {
		c.stdin.Close()
	}
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
		c.cmd.Wait()
	}
	slog.Info("mcp chart client closed")
}
