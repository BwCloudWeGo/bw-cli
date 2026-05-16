package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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
  username: "content-user"
  password: "content-secret"
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
	require.Equal(t, "content-user", cfg.MongoDB.Username)
	require.Equal(t, "content-secret", cfg.MongoDB.Password)
	require.Equal(t, "content", cfg.MongoDB.Database)
	require.Equal(t, "content-service", cfg.MongoDB.AppName)
	require.Equal(t, uint64(2), cfg.MongoDB.MinPoolSize)
	require.Equal(t, uint64(40), cfg.MongoDB.MaxPoolSize)
	require.Equal(t, 8, cfg.MongoDB.ConnectTimeoutSeconds)
	require.Equal(t, 3, cfg.MongoDB.ServerSelectionTimeoutSeconds)
	require.Equal(t, "content-user", cfg.MongoDB.MongoxConfig().Username)
	require.Equal(t, "content-secret", cfg.MongoDB.MongoxConfig().Password)
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

func TestLoadReadsAlipayConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
alipay:
  app_id: "2021000000000000"
  private_key: "private-key-content"
  alipay_public_key: "alipay-public-key-content"
  production: true
  notify_url: "https://api.example.com/payments/alipay/notify"
  return_url: "https://www.example.com/orders/paid"
  encrypt_key: "base64-encrypt-key"
  app_cert_public_key_path: "configs/certs/appCertPublicKey.crt"
  alipay_root_cert_path: "configs/certs/alipayRootCert.crt"
  alipay_cert_public_key_path: "configs/certs/alipayCertPublicKey_RSA2.crt"
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, "2021000000000000", cfg.Alipay.AppID)
	require.Equal(t, "private-key-content", cfg.Alipay.PrivateKey)
	require.Equal(t, "alipay-public-key-content", cfg.Alipay.AlipayPublicKey)
	require.True(t, cfg.Alipay.Production)
	require.Equal(t, "https://api.example.com/payments/alipay/notify", cfg.Alipay.NotifyURL)
	require.Equal(t, "https://www.example.com/orders/paid", cfg.Alipay.ReturnURL)
	require.Equal(t, "base64-encrypt-key", cfg.Alipay.EncryptKey)
	require.Equal(t, "configs/certs/appCertPublicKey.crt", cfg.Alipay.AppCertPublicKeyPath)
	require.Equal(t, "configs/certs/alipayRootCert.crt", cfg.Alipay.AlipayRootCertPath)
	require.Equal(t, "configs/certs/alipayCertPublicKey_RSA2.crt", cfg.Alipay.AlipayCertPublicKeyPath)
}

func TestLoadReadsElasticsearchConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
elasticsearch:
  addresses:
    - https://es-1.example.com:9200
    - https://es-2.example.com:9200
  username: elastic
  password: es-secret
  cloud_id: deployment:cloud-id
  api_key: base64-api-key
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, []string{"https://es-1.example.com:9200", "https://es-2.example.com:9200"}, cfg.Elasticsearch.Addresses)
	require.Equal(t, "elastic", cfg.Elasticsearch.Username)
	require.Equal(t, "es-secret", cfg.Elasticsearch.Password)
	require.Equal(t, "deployment:cloud-id", cfg.Elasticsearch.CloudID)
	require.Equal(t, "base64-api-key", cfg.Elasticsearch.APIKey)
}

func TestLoadReadsElasticsearchAddressesOnly(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
elasticsearch:
  addresses:
    - http://127.0.0.1:9200
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, []string{"http://127.0.0.1:9200"}, cfg.Elasticsearch.Addresses)
	require.Empty(t, cfg.Elasticsearch.Username)
	require.Empty(t, cfg.Elasticsearch.Password)
	require.Empty(t, cfg.Elasticsearch.CloudID)
	require.Empty(t, cfg.Elasticsearch.APIKey)
}

func TestExampleConfigContainsElasticsearchFields(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "configs", "config.yaml"))
	require.NoError(t, err)

	var raw map[string]map[string]any
	require.NoError(t, yaml.Unmarshal(data, &raw))

	elasticsearch, ok := raw["elasticsearch"]
	require.True(t, ok)
	require.Contains(t, elasticsearch, "addresses")
	require.Contains(t, elasticsearch, "username")
	require.Contains(t, elasticsearch, "password")
	require.Contains(t, elasticsearch, "cloud_id")
	require.Contains(t, elasticsearch, "api_key")
}

func TestLoadAppliesRedisAndKafkaDefaults(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`app:
  name: defaults-test
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, "127.0.0.1:6379", cfg.Redis.Addr)
	require.Equal(t, 10, cfg.Redis.PoolSize)
	require.Equal(t, 5*time.Second, cfg.Redis.DialTimeout)
	require.Equal(t, 3*time.Second, cfg.Redis.ReadTimeout)
	require.Equal(t, 3*time.Second, cfg.Redis.WriteTimeout)
	require.Equal(t, "xiaolanshu", cfg.Redis.Lock.KeyPrefix)
	require.Equal(t, 30*time.Second, cfg.Redis.Lock.DefaultTTL)
	require.Equal(t, []string{"127.0.0.1:9092"}, cfg.Kafka.Brokers)
	require.Equal(t, "xiaolanshu-events", cfg.Kafka.Topic)
	require.Equal(t, "xiaolanshu-consumer", cfg.Kafka.GroupID)
	require.Equal(t, kafka.RequireAll, cfg.Kafka.RequiredAcks)
	require.Equal(t, 5*time.Second, cfg.Kafka.DialTimeout)
	require.Equal(t, 10, cfg.Kafka.Producer.MaxAttempts)
	require.Equal(t, 100, cfg.Kafka.Producer.BatchSize)
	require.Equal(t, "none", cfg.Kafka.Producer.Compression)
	require.Equal(t, 1, cfg.Kafka.Consumer.MinBytes)
	require.Equal(t, 10*1024*1024, cfg.Kafka.Consumer.MaxBytes)
	require.Equal(t, "first", cfg.Kafka.Consumer.StartOffset)
}

func TestLoadReadsRedisAndKafkaEnterpriseConfig(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
redis:
  addr: redis.example.com:6379
  username: app
  password: secret
  db: 2
  pool_size: 32
  dial_timeout: 7s
  read_timeout: 4s
  write_timeout: 5s
  lock:
    key_prefix: orders
    default_ttl: 45s
kafka:
  brokers:
    - kafka-1.example.com:9092
    - kafka-2.example.com:9092
  topic: order-events
  group_id: order-worker
  client_id: order-service
  required_acks: all
  dial_timeout: 6s
  producer:
    max_attempts: 6
    batch_size: 80
    batch_bytes: 2097152
    batch_timeout: 50ms
    read_timeout: 8s
    write_timeout: 9s
    async: false
    compression: gzip
    allow_auto_topic_creation: false
  consumer:
    queue_capacity: 256
    min_bytes: 10
    max_bytes: 1048576
    max_wait: 3s
    read_batch_timeout: 4s
    commit_interval: 1s
    heartbeat_interval: 5s
    session_timeout: 35s
    rebalance_timeout: 40s
    start_offset: last
    watch_partition_changes: true
    max_attempts: 5
  sasl:
    enable: true
    mechanism: plain
    username: kafka-user
    password: kafka-secret
  tls:
    enable: true
    insecure_skip_verify: true
    server_name: kafka.example.com
`), 0o644))

	cfg, err := config.Load(path)

	require.NoError(t, err)
	require.Equal(t, "redis.example.com:6379", cfg.Redis.Addr)
	require.Equal(t, "app", cfg.Redis.Username)
	require.Equal(t, "secret", cfg.Redis.Password)
	require.Equal(t, 2, cfg.Redis.DB)
	require.Equal(t, 32, cfg.Redis.PoolSize)
	require.Equal(t, 7*time.Second, cfg.Redis.DialTimeout)
	require.Equal(t, 4*time.Second, cfg.Redis.ReadTimeout)
	require.Equal(t, 5*time.Second, cfg.Redis.WriteTimeout)
	require.Equal(t, "orders", cfg.Redis.Lock.KeyPrefix)
	require.Equal(t, 45*time.Second, cfg.Redis.Lock.DefaultTTL)
	require.Equal(t, []string{"kafka-1.example.com:9092", "kafka-2.example.com:9092"}, cfg.Kafka.Brokers)
	require.Equal(t, "order-events", cfg.Kafka.Topic)
	require.Equal(t, "order-worker", cfg.Kafka.GroupID)
	require.Equal(t, "order-service", cfg.Kafka.ClientID)
	require.Equal(t, kafka.RequireAll, cfg.Kafka.RequiredAcks)
	require.Equal(t, 6*time.Second, cfg.Kafka.DialTimeout)
	require.Equal(t, 6, cfg.Kafka.Producer.MaxAttempts)
	require.Equal(t, 80, cfg.Kafka.Producer.BatchSize)
	require.Equal(t, int64(2097152), cfg.Kafka.Producer.BatchBytes)
	require.Equal(t, 50*time.Millisecond, cfg.Kafka.Producer.BatchTimeout)
	require.Equal(t, 8*time.Second, cfg.Kafka.Producer.ReadTimeout)
	require.Equal(t, 9*time.Second, cfg.Kafka.Producer.WriteTimeout)
	require.Equal(t, "gzip", cfg.Kafka.Producer.Compression)
	require.Equal(t, 256, cfg.Kafka.Consumer.QueueCapacity)
	require.Equal(t, 10, cfg.Kafka.Consumer.MinBytes)
	require.Equal(t, 1048576, cfg.Kafka.Consumer.MaxBytes)
	require.Equal(t, 3*time.Second, cfg.Kafka.Consumer.MaxWait)
	require.Equal(t, 4*time.Second, cfg.Kafka.Consumer.ReadBatchTimeout)
	require.Equal(t, time.Second, cfg.Kafka.Consumer.CommitInterval)
	require.Equal(t, 5*time.Second, cfg.Kafka.Consumer.HeartbeatInterval)
	require.Equal(t, 35*time.Second, cfg.Kafka.Consumer.SessionTimeout)
	require.Equal(t, 40*time.Second, cfg.Kafka.Consumer.RebalanceTimeout)
	require.Equal(t, "last", cfg.Kafka.Consumer.StartOffset)
	require.True(t, cfg.Kafka.Consumer.WatchPartitionChanges)
	require.Equal(t, 5, cfg.Kafka.Consumer.MaxAttempts)
	require.True(t, cfg.Kafka.SASL.Enable)
	require.Equal(t, "plain", cfg.Kafka.SASL.Mechanism)
	require.Equal(t, "kafka-user", cfg.Kafka.SASL.Username)
	require.Equal(t, "kafka-secret", cfg.Kafka.SASL.Password)
	require.True(t, cfg.Kafka.TLS.Enable)
	require.True(t, cfg.Kafka.TLS.InsecureSkipVerify)
	require.Equal(t, "kafka.example.com", cfg.Kafka.TLS.ServerName)
}

func TestInitGlobalSetsProcessWideConfig(t *testing.T) {
	previous := config.GlobalConfig
	defer func() { config.GlobalConfig = previous }()

	tmp := t.TempDir()
	path := filepath.Join(tmp, "config.yaml")
	require.NoError(t, os.WriteFile(path, []byte(`
app:
  name: global-test
`), 0o644))

	require.NoError(t, config.InitGlobal(path))

	require.NotNil(t, config.GlobalConfig)
	require.Equal(t, "global-test", config.MustGlobal().App.Name)
}
