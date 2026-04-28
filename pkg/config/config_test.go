package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/config"
)

func TestLoadReadsPostgreSQLAndMongoDBConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
database:
  driver: postgres
postgresql:
  dsn: "host=127.0.0.1 user=app password=secret dbname=app port=5432 sslmode=disable"
  max_idle_conns: 7
  max_open_conns: 80
  conn_max_lifetime_seconds: 600
mongodb:
  uri: "mongodb://mongo:27017"
  database: "content"
  app_name: "content-service"
  min_pool_size: 2
  max_pool_size: 40
  connect_timeout_seconds: 8
  server_selection_timeout_seconds: 3
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, "postgres", cfg.Database.Driver)
	require.Equal(t, "host=127.0.0.1 user=app password=secret dbname=app port=5432 sslmode=disable", cfg.PostgreSQL.DSN)
	require.Equal(t, 7, cfg.PostgreSQL.MaxIdleConns)
	require.Equal(t, 80, cfg.PostgreSQL.MaxOpenConns)
	require.Equal(t, 600, cfg.PostgreSQL.ConnMaxLifetimeSeconds)
	require.Equal(t, "mongodb://mongo:27017", cfg.MongoDB.URI)
	require.Equal(t, "content", cfg.MongoDB.Database)
	require.Equal(t, "content-service", cfg.MongoDB.AppName)
	require.Equal(t, uint64(2), cfg.MongoDB.MinPoolSize)
	require.Equal(t, uint64(40), cfg.MongoDB.MaxPoolSize)
	require.Equal(t, 8, cfg.MongoDB.ConnectTimeoutSeconds)
	require.Equal(t, 3, cfg.MongoDB.ServerSelectionTimeoutSeconds)
}
