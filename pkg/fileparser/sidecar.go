package fileparser

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"innovation-incubation-platform-backend/config"
)

type Sidecar struct {
	cmd  *exec.Cmd
	port int
	cfg  config.FileParserConfig
}

func NewSidecar(cfg config.FileParserConfig) *Sidecar {
	return &Sidecar{cfg: cfg}
}

func (s *Sidecar) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("sidecar disabled by config")
	}

	// 找随机空闲端口
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return fmt.Errorf("find free port: %w", err)
	}
	s.port = listener.(*net.TCPListener).Addr().(*net.TCPAddr).Port
	listener.Close()

	pythonPath := filepath.Join(s.cfg.VenvPath, "bin", "python")
	if _, err := os.Stat(pythonPath); err != nil {
		// Windows: check python.exe instead
		pythonPath = filepath.Join(s.cfg.VenvPath, "Scripts", "python.exe")
		if _, err2 := os.Stat(pythonPath); err2 != nil {
			return fmt.Errorf("venv python not found: check %s or %s",
				filepath.Join(s.cfg.VenvPath, "bin", "python"),
				filepath.Join(s.cfg.VenvPath, "Scripts", "python.exe"))
		}
	}

	portStr := strconv.Itoa(s.port)
	s.cmd = exec.Command(pythonPath, s.cfg.ScriptPath, portStr)
	s.cmd.Stderr = os.Stderr
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("sidecar start failed: %w", err)
	}

	// 轮询等待 TCP 就绪
	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			sidecarBaseURL = fmt.Sprintf("http://127.0.0.1:%d", s.port)
			globalClient = &http.Client{
				Transport: &http.Transport{
					DialContext: (&net.Dialer{}).DialContext,
				},
				Timeout: time.Duration(s.cfg.TimeoutSec) * time.Second,
			}
			slog.Info("file parser sidecar ready", "addr", addr)
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("sidecar did not become ready within 5s")
}

func (s *Sidecar) Stop() error {
	if s.cmd == nil || s.cmd.Process == nil {
		return nil
	}
	if err := s.cmd.Process.Signal(os.Interrupt); err != nil {
		slog.Warn("sidecar signal failed, killing", "error", err)
		s.cmd.Process.Kill()
	}
	done := make(chan error, 1)
	go func() {
		done <- s.cmd.Wait()
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		s.cmd.Process.Kill()
		<-done
	}
	return nil
}
