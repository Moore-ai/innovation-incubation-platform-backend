package database

import (
	"io"
	"log/slog"
	"os"
	"time"
)

func InitFileLogger(logDir string) (*os.File, error) {
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, err
	}
	filename := time.Now().Format("2006-01-02_150405.000000") + ".log"
	f, err := os.OpenFile(logDir+"/"+filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	w := io.MultiWriter(os.Stderr, f)
	slog.SetDefault(slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelInfo})))
	return f, nil
}
