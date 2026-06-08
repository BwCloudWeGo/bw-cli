package nacosx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWithDefaultsKeepsNacosDisabledAndFillsConnectionDefaults(t *testing.T) {
	cfg := WithDefaults(Config{})

	require.False(t, cfg.Enabled)
	require.Equal(t, "127.0.0.1", cfg.ServerAddr)
	require.Equal(t, uint64(8848), cfg.ServerPort)
	require.Equal(t, "DEFAULT_GROUP", cfg.Group)
	require.Equal(t, "xiaolanshu.yaml", cfg.DataID)
	require.Equal(t, uint64(5000), cfg.TimeoutMs)
}

func TestLoadConfigReturnsEmptyWhenDisabled(t *testing.T) {
	content, err := LoadConfig(Config{Enabled: false})

	require.NoError(t, err)
	require.Empty(t, content)
}
