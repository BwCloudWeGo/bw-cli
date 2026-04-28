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

func TestLoadReadsFileStorageConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
file_storage:
  provider: minio
  max_size_mb: 20
  object_prefix: uploads
  public_base_url: https://cdn.example.com
  allowed_extensions:
    - .pdf
    - .png
  allowed_content_types:
    - application/pdf
    - image/png
  minio:
    endpoint: 127.0.0.1:9000
    access_key_id: minioadmin
    secret_access_key: minioadmin
    bucket: app-files
    use_ssl: false
  oss:
    endpoint: https://oss-cn-hangzhou.aliyuncs.com
    access_key_id: oss-ak
    access_key_secret: oss-sk
    bucket: app-oss
  qiniu:
    access_key: qiniu-ak
    secret_key: qiniu-sk
    bucket: app-qiniu
    region: z0
    use_https: true
  cos:
    secret_id: cos-id
    secret_key: cos-key
    bucket: app-cos-1250000000
    region: ap-guangzhou
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, "minio", cfg.FileStorage.Provider)
	require.Equal(t, int64(20), cfg.FileStorage.MaxSizeMB)
	require.Equal(t, "uploads", cfg.FileStorage.ObjectPrefix)
	require.Equal(t, "https://cdn.example.com", cfg.FileStorage.PublicBaseURL)
	require.Equal(t, []string{".pdf", ".png"}, cfg.FileStorage.AllowedExtensions)
	require.Equal(t, []string{"application/pdf", "image/png"}, cfg.FileStorage.AllowedContentTypes)
	require.Equal(t, "127.0.0.1:9000", cfg.FileStorage.MinIO.Endpoint)
	require.Equal(t, "app-files", cfg.FileStorage.MinIO.Bucket)
	require.Equal(t, "https://oss-cn-hangzhou.aliyuncs.com", cfg.FileStorage.OSS.Endpoint)
	require.Equal(t, "app-oss", cfg.FileStorage.OSS.Bucket)
	require.Equal(t, "qiniu-ak", cfg.FileStorage.Qiniu.AccessKey)
	require.Equal(t, "z0", cfg.FileStorage.Qiniu.Region)
	require.True(t, cfg.FileStorage.Qiniu.UseHTTPS)
	require.Equal(t, "cos-id", cfg.FileStorage.COS.SecretID)
	require.Equal(t, "ap-guangzhou", cfg.FileStorage.COS.Region)
}
