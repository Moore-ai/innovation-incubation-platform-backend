package fileparser

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"innovation-incubation-platform-backend/config"
)

type Sidecar struct {
	cmd      *exec.Cmd
	sockPath string
	cfg      config.FileParserConfig
}

func NewSidecar(cfg config.FileParserConfig) *Sidecar {
	return &Sidecar{cfg: cfg}
}

func (s *Sidecar) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		return fmt.Errorf("sidecar disabled by config")
	}

	randBytes := make([]byte, 8)
	if _, err := rand.Read(randBytes); err != nil {
		return fmt.Errorf("generate socket id: %w", err)
	}
	sockFile := "fileparser-" + hex.EncodeToString(randBytes) + ".sock"
	sockDir := s.cfg.SocketDir
	if sockDir == "" {
		sockDir = os.TempDir()
	}
	s.sockPath = filepath.Join(sockDir, sockFile)

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

	s.cmd = exec.Command(pythonPath, s.cfg.ScriptPath, s.sockPath)
	s.cmd.Stderr = os.Stderr
	if err := s.cmd.Start(); err != nil {
		return fmt.Errorf("sidecar start failed: %w", err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", s.sockPath, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			globalClient = &http.Client{
				Transport: &http.Transport{
					DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
						return net.Dial("unix", s.sockPath)
					},
				},
				Timeout: time.Duration(s.cfg.TimeoutSec) * time.Second,
			}
			slog.Info("file parser sidecar ready", "socket", s.sockPath)
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
	if s.sockPath != "" {
		os.Remove(s.sockPath)
	}
	return nil
}
