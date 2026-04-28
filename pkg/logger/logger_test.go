package logger_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
)

func TestDefaultConfigRetainsLogFilesForSevenDays(t *testing.T) {
	cfg := logger.DefaultConfig("user-service")

	require.Equal(t, "user-service", cfg.Service)
	require.Equal(t, "info", cfg.Level)
	require.Equal(t, "logs/app.log", cfg.File.Filename)
	require.Equal(t, 7, cfg.File.MaxAgeDays)
	require.True(t, cfg.File.Compress)
}

func TestWithDailyFileNameUsesServiceAndCurrentDate(t *testing.T) {
	cfg := logger.DefaultConfig("gateway")
	now := time.Date(2026, 4, 28, 12, 0, 0, 0, time.UTC)

	cfg = logger.WithDailyFileName(cfg, now)

	require.Equal(t, "logs/gateway-2026-04-28.log", cfg.File.Filename)
}

func TestNewLoggerAttachesCommonServiceDimensions(t *testing.T) {
	cfg := logger.DefaultConfig("note-service")
	cfg.Environment = "test"
	cfg.File.Enabled = false

	log, err := logger.New(cfg)
	require.NoError(t, err)
	require.NotNil(t, log)

	// Smoke test for required common dimensions. The logger should be ready to
	// write structured logs with service/env fields already attached.
	log.Info("logger initialized")
}
