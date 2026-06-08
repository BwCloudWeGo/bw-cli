package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceHelpersUseConfiguredValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
services:
  user:
    name: account-service
    port: 9201
    target: user:9201
`), 0o644))

	cfg, err := Load(path)

	require.NoError(t, err)
	require.Equal(t, "account-service", cfg.ServiceName("user"))
	require.Equal(t, 9201, cfg.ServicePort("user", 9001))
	require.Equal(t, "user:9201", cfg.ServiceTarget("user"))
}

func TestServiceTargetUsesConfiguredPortWhenTargetMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
services:
  comment:
    name: comment-service
    port: 9103
`), 0o644))

	cfg, err := Load(path)

	require.NoError(t, err)
	require.Equal(t, "comment-service", cfg.ServiceName("comment"))
	require.Equal(t, 9103, cfg.ServicePort("comment", 9103))
	require.Equal(t, "127.0.0.1:9103", cfg.ServiceTarget("comment"))
}

func TestServiceHelpersFallbackWhenServiceMissing(t *testing.T) {
	cfg := &Config{}

	require.Equal(t, "comment-service", cfg.ServiceName("comment"))
	require.Equal(t, 9103, cfg.ServicePort("comment", 9103))
	require.Empty(t, cfg.ServiceTarget("comment"))
}
