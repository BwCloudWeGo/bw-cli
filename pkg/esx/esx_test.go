package esx_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/esx"
)

func TestDefaultConfig(t *testing.T) {
	cfg := esx.DefaultConfig()

	require.Equal(t, []string{"http://127.0.0.1:9200"}, cfg.Addresses)
	require.Empty(t, cfg.Username)
	require.Empty(t, cfg.Password)
}
