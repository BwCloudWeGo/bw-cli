package mysqlx_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/mysqlx"
)

func TestDefaultConfig(t *testing.T) {
	cfg := mysqlx.DefaultConfig()

	require.Empty(t, cfg.DSN)
	require.Equal(t, 10, cfg.MaxIdleConns)
	require.Equal(t, 100, cfg.MaxOpenConns)
	require.Equal(t, time.Hour, cfg.ConnMaxLifetime)
}
