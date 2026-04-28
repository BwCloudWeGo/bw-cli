package redisx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/redisx"
)

func TestDefaultConfig(t *testing.T) {
	cfg := redisx.DefaultConfig()

	require.Equal(t, "127.0.0.1:6379", cfg.Addr)
	require.Equal(t, 0, cfg.DB)
	require.Equal(t, 10, cfg.PoolSize)
}
