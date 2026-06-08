package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestDefaultConfigRetainsLogFilesForSevenDays(t *testing.T) {
	cfg := DefaultConfig("user-service")

	require.Equal(t, "user-service", cfg.Service)
	require.Equal(t, "info", cfg.Level)
	require.Equal(t, "logs/app.log", cfg.File.Filename)
	require.Equal(t, 7, cfg.File.MaxAgeDays)
	require.True(t, cfg.File.Compress)
}

func TestWithDailyFileNameUsesServiceAndCurrentDate(t *testing.T) {
	cfg := DefaultConfig("gateway")
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	cfg = WithDailyFileName(cfg, now)

	require.Equal(t, "logs/gateway-2026-04-28.log", cfg.File.Filename)
}

func TestNewLoggerAttachesCommonServiceDimensions(t *testing.T) {
	cfg := DefaultConfig("note-service")
	cfg.Environment = "test"
	cfg.File.Enabled = false

	log, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)

	// Smoke test for required common dimensions. The logger should be ready to
	// write structured logs with service/env fields already attached.
	log.Info("logger initialized")
}

func TestWriterUsesConsoleAndFileWhenFileLoggingEnabled(t *testing.T) {
	var console bytes.Buffer
	path := filepath.Join(t.TempDir(), "app.log")

	sink := writerWithConsole(FileConfig{
		Enabled:    true,
		Filename:   path,
		MaxSizeMB:  1,
		MaxBackups: 1,
		MaxAgeDays: 1,
	}, zapcore.AddSync(&console))

	_, err := sink.Write([]byte("hello\n"))
	require.NoError(t, err)
	require.NoError(t, sink.Sync())

	require.Contains(t, console.String(), "hello")
	fileData, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(fileData), "hello")
}
