package postgresx_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	gormlogger "gorm.io/gorm/logger"

	"github.com/BwCloudWeGo/bw-cli/pkg/postgresx"
)

func TestDefaultConfigUsesSafePoolDefaults(t *testing.T) {
	cfg := postgresx.DefaultConfig()

	require.Empty(t, cfg.DSN)
	require.Equal(t, 10, cfg.MaxIdleConns)
	require.Equal(t, 100, cfg.MaxOpenConns)
	require.Equal(t, time.Hour, cfg.ConnMaxLifetime)
	require.Equal(t, gormlogger.Warn, cfg.LogLevel)
}

func TestOpenRequiresDSN(t *testing.T) {
	db, err := postgresx.Open(postgresx.Config{})

	require.Nil(t, db)
	require.EqualError(t, err, "postgres dsn is required")
}
