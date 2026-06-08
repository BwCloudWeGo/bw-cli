package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

const runNoteMongoDBExampleEnv = "RUN_NOTE_MONGODB_EXAMPLE"

func TestRunMongoDocumentStoreExampleUsesCurrentConfig(t *testing.T) {
	if os.Getenv(runNoteMongoDBExampleEnv) != "true" {
		t.Skip("set RUN_NOTE_MONGODB_EXAMPLE=true to run this MongoDB example against configs/config.yaml")
	}

	previous := config.GlobalConfig
	defer func() { config.GlobalConfig = previous }()

	configPath := filepath.Join("..", "..", "configs", "config.yaml")
	require.NoError(t, config.InitGlobal(configPath))
	cfg := config.MustGlobal()

	document, err := runMongoDocumentStoreExample(context.Background(), cfg, zap.NewNop())
	require.NoError(t, err)
	require.NotNil(t, document)
	require.NotEmpty(t, document.ID)
	require.Equal(t, cfg.ServiceName("note"), document.Service)
}
