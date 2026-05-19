package config

import (
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/spf13/viper"

	"github.com/BwCloudWeGo/bw-cli/pkg/alipayx"
	"github.com/BwCloudWeGo/bw-cli/pkg/esx"
	"github.com/BwCloudWeGo/bw-cli/pkg/filex"
	"github.com/BwCloudWeGo/bw-cli/pkg/kafkax"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
	"github.com/BwCloudWeGo/bw-cli/pkg/mysqlx"
	"github.com/BwCloudWeGo/bw-cli/pkg/nacosx"
	"github.com/BwCloudWeGo/bw-cli/pkg/postgresx"
	"github.com/BwCloudWeGo/bw-cli/pkg/redisx"
)

var remoteConfigLoader = nacosx.LoadConfig

// Source identifies where the process loaded its application config from.
type Source string

const (
	SourceLocal Source = "local"
	SourceNacos Source = "nacos"
)

// AppConfig contains project and service identity values shared by all processes.
type AppConfig struct {
	Name string `mapstructure:"name" yaml:"name"`
	Env  string `mapstructure:"env" yaml:"env"`
}

// HTTPConfig controls the Gin gateway listener and server timeouts.
type HTTPConfig struct {
	Host                string `mapstructure:"host" yaml:"host"`
	Port                int    `mapstructure:"port" yaml:"port"`
	ReadTimeoutSeconds  int    `mapstructure:"read_timeout_seconds" yaml:"read_timeout_seconds"`
	WriteTimeoutSeconds int    `mapstructure:"write_timeout_seconds" yaml:"write_timeout_seconds"`
}

// GRPCConfig controls gRPC server ports and gateway client targets.
type GRPCConfig struct {
	Host string `mapstructure:"host" yaml:"host"`
}

// ServiceConfig describes one process-level service instance.
type ServiceConfig struct {
	Name   string `mapstructure:"name" yaml:"name"`
	Port   int    `mapstructure:"port" yaml:"port"`
	Target string `mapstructure:"target" yaml:"target"`
}

// DatabaseConfig selects the active database driver used by demo services.
type DatabaseConfig struct {
	Driver string `mapstructure:"driver" yaml:"driver"`
	DSN    string `mapstructure:"dsn" yaml:"dsn"`
}

// MySQLConfig contains the MySQL DSN and sql.DB connection pool settings.
type MySQLConfig struct {
	DSN                    string `mapstructure:"dsn" yaml:"dsn"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns           int    `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	ConnMaxLifetimeSeconds int    `mapstructure:"conn_max_lifetime_seconds" yaml:"conn_max_lifetime_seconds"`
}

// ConnMaxLifetime converts the YAML seconds value to a duration used by sql.DB.
func (cfg MySQLConfig) ConnMaxLifetime() time.Duration {
	return time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second
}

// PostgreSQLConfig contains the PostgreSQL DSN and sql.DB connection pool settings.
type PostgreSQLConfig struct {
	DSN                    string `mapstructure:"dsn" yaml:"dsn"`
	MaxIdleConns           int    `mapstructure:"max_idle_conns" yaml:"max_idle_conns"`
	MaxOpenConns           int    `mapstructure:"max_open_conns" yaml:"max_open_conns"`
	ConnMaxLifetimeSeconds int    `mapstructure:"conn_max_lifetime_seconds" yaml:"conn_max_lifetime_seconds"`
}

// ConnMaxLifetime converts the YAML seconds value to a duration used by sql.DB.
func (cfg PostgreSQLConfig) ConnMaxLifetime() time.Duration {
	return time.Duration(cfg.ConnMaxLifetimeSeconds) * time.Second
}

// MongoDBConfig contains MongoDB client, database and pool settings loaded from YAML.
type MongoDBConfig struct {
	URI                           string `mapstructure:"uri" yaml:"uri"`
	Username                      string `mapstructure:"username" yaml:"username"`
	Password                      string `mapstructure:"password" yaml:"password"`
	Database                      string `mapstructure:"database" yaml:"database"`
	AppName                       string `mapstructure:"app_name" yaml:"app_name"`
	MinPoolSize                   uint64 `mapstructure:"min_pool_size" yaml:"min_pool_size"`
	MaxPoolSize                   uint64 `mapstructure:"max_pool_size" yaml:"max_pool_size"`
	ConnectTimeoutSeconds         int    `mapstructure:"connect_timeout_seconds" yaml:"connect_timeout_seconds"`
	ServerSelectionTimeoutSeconds int    `mapstructure:"server_selection_timeout_seconds" yaml:"server_selection_timeout_seconds"`
}

// ConnectTimeout converts the YAML seconds value to a MongoDB connect timeout.
func (cfg MongoDBConfig) ConnectTimeout() time.Duration {
	return time.Duration(cfg.ConnectTimeoutSeconds) * time.Second
}

// ServerSelectionTimeout converts the YAML seconds value to a MongoDB server selection timeout.
func (cfg MongoDBConfig) ServerSelectionTimeout() time.Duration {
	return time.Duration(cfg.ServerSelectionTimeoutSeconds) * time.Second
}

// MongoxConfig converts YAML configuration into the shared MongoDB client config.
func (cfg MongoDBConfig) MongoxConfig() mongox.Config {
	return mongox.Config{
		URI:                    cfg.URI,
		Username:               cfg.Username,
		Password:               cfg.Password,
		Database:               cfg.Database,
		AppName:                cfg.AppName,
		MinPoolSize:            cfg.MinPoolSize,
		MaxPoolSize:            cfg.MaxPoolSize,
		ConnectTimeout:         cfg.ConnectTimeout(),
		ServerSelectionTimeout: cfg.ServerSelectionTimeout(),
	}
}

// MiddlewareConfig groups HTTP middleware configuration loaded from YAML.
type MiddlewareConfig struct {
	CORS middleware.CORSConfig `mapstructure:"cors" yaml:"cors"`
	JWT  middleware.JWTConfig  `mapstructure:"jwt" yaml:"jwt"`
}

// Config is the root application configuration loaded by each process.
type Config struct {
	Source        Source                   `mapstructure:"-" yaml:"-"`
	App           AppConfig                `mapstructure:"app" yaml:"app"`
	HTTP          HTTPConfig               `mapstructure:"http" yaml:"http"`
	GRPC          GRPCConfig               `mapstructure:"grpc" yaml:"grpc"`
	Services      map[string]ServiceConfig `mapstructure:"services" yaml:"services"`
	Database      DatabaseConfig           `mapstructure:"database" yaml:"database"`
	MySQL         MySQLConfig              `mapstructure:"mysql" yaml:"mysql"`
	PostgreSQL    PostgreSQLConfig         `mapstructure:"postgresql" yaml:"postgresql"`
	MongoDB       MongoDBConfig            `mapstructure:"mongodb" yaml:"mongodb"`
	FileStorage   filex.Config             `mapstructure:"file_storage" yaml:"file_storage"`
	Redis         redisx.Config            `mapstructure:"redis" yaml:"redis"`
	Elasticsearch esx.Config               `mapstructure:"elasticsearch" yaml:"elasticsearch"`
	Kafka         kafkax.Config            `mapstructure:"kafka" yaml:"kafka"`
	Alipay        alipayx.Config           `mapstructure:"alipay" yaml:"alipay"`
	Nacos         nacosx.Config            `mapstructure:"nacos" yaml:"nacos"`
	Middleware    MiddlewareConfig         `mapstructure:"middleware" yaml:"middleware"`
	Log           logger.Config            `mapstructure:"log" yaml:"log"`
}

// Service returns a named service config with a predictable fallback.
func (cfg *Config) Service(key string) ServiceConfig {
	if cfg == nil {
		return ServiceConfig{}
	}
	key = strings.TrimSpace(key)
	if svc, ok := cfg.Services[key]; ok {
		if svc.Name == "" {
			svc.Name = key + "-service"
		}
		return svc
	}
	return ServiceConfig{Name: key + "-service"}
}

// ServiceName returns the configured service name for logs and observability.
func (cfg *Config) ServiceName(key string) string {
	return cfg.Service(key).Name
}

// ServicePort returns the configured service port, or fallback when unset.
func (cfg *Config) ServicePort(key string, fallback int) int {
	if port := cfg.Service(key).Port; port > 0 {
		return port
	}
	return fallback
}

// ServiceTarget returns the configured gRPC target, or localhost:<configured port>.
func (cfg *Config) ServiceTarget(key string) string {
	svc := cfg.Service(key)
	if target := strings.TrimSpace(svc.Target); target != "" {
		return target
	}
	if svc.Port > 0 {
		return "127.0.0.1:" + strconv.Itoa(svc.Port)
	}
	return ""
}

// UsingNacos reports whether this process is running with config loaded from Nacos.
func (cfg *Config) UsingNacos() bool {
	return cfg != nil && cfg.Source == SourceNacos
}

// PrintSourceNotice writes a startup notice when runtime config comes from Nacos.
func PrintSourceNotice(cfg *Config, out io.Writer) {
	if cfg == nil || out == nil || !cfg.UsingNacos() {
		return
	}
	nacos := cfg.Nacos
	fmt.Fprintf(out, "\n[Config]\n")
	fmt.Fprintf(out, "  source: nacos\n")
	fmt.Fprintf(out, "  server: %s:%d\n", nacos.ServerAddr, nacos.ServerPort)
	if strings.TrimSpace(nacos.NamespaceID) != "" {
		fmt.Fprintf(out, "  namespace_id: %s\n", nacos.NamespaceID)
	}
	fmt.Fprintf(out, "  group: %s\n", nacos.Group)
	fmt.Fprintf(out, "  data_id: %s\n\n", nacos.DataID)
}

// Load reads YAML configuration from local files or Nacos.
func Load(path string) (*Config, error) {
	nacosCfg, err := loadNacosConfig(path)
	if err != nil {
		return nil, err
	}
	if nacosCfg.Enabled {
		return loadFromNacos(nacosCfg, path)
	}
	return loadFromLocal(path)
}

func loadFromLocal(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")

	setDefaults(v)
	if path != "" {
		v.SetConfigFile(path)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath("configs")
		v.AddConfigPath(".")
	}
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(decodeHook())); err != nil {
		return nil, err
	}
	cfg.Source = SourceLocal
	return &cfg, nil
}

func loadFromNacos(nacosCfg nacosx.Config, fallbackPath string) (*Config, error) {
	remoteYAML, err := remoteConfigLoader(nacosCfg)
	if err != nil {
		if nacosCfg.FailFast {
			return nil, fmt.Errorf("load nacos config: %w", err)
		}
		return loadFromLocal(fallbackPath)
	}
	if strings.TrimSpace(remoteYAML) == "" {
		if nacosCfg.FailFast {
			return nil, fmt.Errorf("load nacos config: empty config data")
		}
		return loadFromLocal(fallbackPath)
	}
	v := viper.New()
	v.SetConfigType("yaml")
	setDefaults(v)
	if err := v.ReadConfig(strings.NewReader(remoteYAML)); err != nil {
		return nil, fmt.Errorf("parse nacos config: %w", err)
	}
	var cfg Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(decodeHook())); err != nil {
		return nil, err
	}
	cfg.Source = SourceNacos
	cfg.Nacos = nacosCfg
	return &cfg, nil
}

func loadNacosConfig(configPath string) (nacosx.Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	setNacosDefaults(v)
	if configPath != "" {
		v.SetConfigFile(filepath.Join(filepath.Dir(configPath), "nacos.yaml"))
	} else {
		v.SetConfigName("nacos")
		v.AddConfigPath("configs")
		v.AddConfigPath(".")
	}
	if err := v.ReadInConfig(); err != nil {
		if !isConfigFileNotFound(err) {
			return nacosx.Config{}, err
		}
	}
	var cfg nacosx.Config
	if err := v.Unmarshal(&cfg, viper.DecodeHook(decodeHook())); err != nil {
		return nacosx.Config{}, err
	}
	return nacosx.WithDefaults(cfg), nil
}

func isConfigFileNotFound(err error) bool {
	if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		return true
	}
	return strings.Contains(err.Error(), "no such file or directory")
}

func decodeHook() mapstructure.DecodeHookFunc {
	return mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		mapstructure.StringToSliceHookFunc(","),
		mapstructure.TextUnmarshallerHookFunc(),
	)
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "xiaolanshu")
	v.SetDefault("app.env", "local")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout_seconds", 5)
	v.SetDefault("http.write_timeout_seconds", 10)
	v.SetDefault("grpc.host", "0.0.0.0")
	v.SetDefault("services.gateway.name", "gateway")
	v.SetDefault("services.user.name", "user-service")
	v.SetDefault("services.user.port", 9001)
	v.SetDefault("services.user.target", "127.0.0.1:9001")
	v.SetDefault("services.note.name", "note-service")
	v.SetDefault("services.note.port", 9002)
	v.SetDefault("services.note.target", "127.0.0.1:9002")
	v.SetDefault("database.driver", "sqlite")
	v.SetDefault("database.dsn", "data/xiaolanshu.db")
	v.SetDefault("mysql.dsn", mysqlx.DefaultConfig().DSN)
	v.SetDefault("mysql.max_idle_conns", mysqlx.DefaultConfig().MaxIdleConns)
	v.SetDefault("mysql.max_open_conns", mysqlx.DefaultConfig().MaxOpenConns)
	v.SetDefault("mysql.conn_max_lifetime_seconds", int(mysqlx.DefaultConfig().ConnMaxLifetime/time.Second))
	v.SetDefault("postgresql.dsn", postgresx.DefaultConfig().DSN)
	v.SetDefault("postgresql.max_idle_conns", postgresx.DefaultConfig().MaxIdleConns)
	v.SetDefault("postgresql.max_open_conns", postgresx.DefaultConfig().MaxOpenConns)
	v.SetDefault("postgresql.conn_max_lifetime_seconds", int(postgresx.DefaultConfig().ConnMaxLifetime/time.Second))
	v.SetDefault("mongodb.uri", mongox.DefaultConfig().URI)
	v.SetDefault("mongodb.username", mongox.DefaultConfig().Username)
	v.SetDefault("mongodb.password", mongox.DefaultConfig().Password)
	v.SetDefault("mongodb.database", mongox.DefaultConfig().Database)
	v.SetDefault("mongodb.app_name", mongox.DefaultConfig().AppName)
	v.SetDefault("mongodb.min_pool_size", mongox.DefaultConfig().MinPoolSize)
	v.SetDefault("mongodb.max_pool_size", mongox.DefaultConfig().MaxPoolSize)
	v.SetDefault("mongodb.connect_timeout_seconds", int(mongox.DefaultConfig().ConnectTimeout/time.Second))
	v.SetDefault("mongodb.server_selection_timeout_seconds", int(mongox.DefaultConfig().ServerSelectionTimeout/time.Second))
	v.SetDefault("file_storage.provider", filex.DefaultConfig().Provider)
	v.SetDefault("file_storage.max_size_mb", filex.DefaultConfig().MaxSizeMB)
	v.SetDefault("file_storage.object_prefix", filex.DefaultConfig().ObjectPrefix)
	v.SetDefault("file_storage.public_base_url", filex.DefaultConfig().PublicBaseURL)
	v.SetDefault("file_storage.allowed_extensions", filex.DefaultConfig().AllowedExtensions)
	v.SetDefault("file_storage.allowed_content_types", filex.DefaultConfig().AllowedContentTypes)
	v.SetDefault("redis.addr", redisx.DefaultConfig().Addr)
	v.SetDefault("redis.username", redisx.DefaultConfig().Username)
	v.SetDefault("redis.password", redisx.DefaultConfig().Password)
	v.SetDefault("redis.db", redisx.DefaultConfig().DB)
	v.SetDefault("redis.pool_size", redisx.DefaultConfig().PoolSize)
	v.SetDefault("redis.dial_timeout", redisx.DefaultConfig().DialTimeout)
	v.SetDefault("redis.read_timeout", redisx.DefaultConfig().ReadTimeout)
	v.SetDefault("redis.write_timeout", redisx.DefaultConfig().WriteTimeout)
	v.SetDefault("redis.lock.key_prefix", redisx.DefaultConfig().Lock.KeyPrefix)
	v.SetDefault("redis.lock.default_ttl", redisx.DefaultConfig().Lock.DefaultTTL)
	v.SetDefault("elasticsearch.addresses", esx.DefaultConfig().Addresses)
	v.SetDefault("elasticsearch.username", esx.DefaultConfig().Username)
	v.SetDefault("elasticsearch.password", esx.DefaultConfig().Password)
	v.SetDefault("elasticsearch.cloud_id", esx.DefaultConfig().CloudID)
	v.SetDefault("elasticsearch.api_key", esx.DefaultConfig().APIKey)
	v.SetDefault("kafka.brokers", kafkax.DefaultConfig().Brokers)
	v.SetDefault("kafka.topic", kafkax.DefaultConfig().Topic)
	v.SetDefault("kafka.group_id", kafkax.DefaultConfig().GroupID)
	v.SetDefault("kafka.client_id", kafkax.DefaultConfig().ClientID)
	v.SetDefault("kafka.required_acks", kafkax.DefaultConfig().RequiredAcks)
	v.SetDefault("kafka.dial_timeout", kafkax.DefaultConfig().DialTimeout)
	v.SetDefault("kafka.producer.max_attempts", kafkax.DefaultConfig().Producer.MaxAttempts)
	v.SetDefault("kafka.producer.batch_size", kafkax.DefaultConfig().Producer.BatchSize)
	v.SetDefault("kafka.producer.batch_bytes", kafkax.DefaultConfig().Producer.BatchBytes)
	v.SetDefault("kafka.producer.batch_timeout", kafkax.DefaultConfig().Producer.BatchTimeout)
	v.SetDefault("kafka.producer.read_timeout", kafkax.DefaultConfig().Producer.ReadTimeout)
	v.SetDefault("kafka.producer.write_timeout", kafkax.DefaultConfig().Producer.WriteTimeout)
	v.SetDefault("kafka.producer.async", kafkax.DefaultConfig().Producer.Async)
	v.SetDefault("kafka.producer.compression", kafkax.DefaultConfig().Producer.Compression)
	v.SetDefault("kafka.producer.allow_auto_topic_creation", kafkax.DefaultConfig().Producer.AllowAutoTopicCreation)
	v.SetDefault("kafka.consumer.queue_capacity", kafkax.DefaultConfig().Consumer.QueueCapacity)
	v.SetDefault("kafka.consumer.min_bytes", kafkax.DefaultConfig().Consumer.MinBytes)
	v.SetDefault("kafka.consumer.max_bytes", kafkax.DefaultConfig().Consumer.MaxBytes)
	v.SetDefault("kafka.consumer.max_wait", kafkax.DefaultConfig().Consumer.MaxWait)
	v.SetDefault("kafka.consumer.read_batch_timeout", kafkax.DefaultConfig().Consumer.ReadBatchTimeout)
	v.SetDefault("kafka.consumer.commit_interval", kafkax.DefaultConfig().Consumer.CommitInterval)
	v.SetDefault("kafka.consumer.heartbeat_interval", kafkax.DefaultConfig().Consumer.HeartbeatInterval)
	v.SetDefault("kafka.consumer.session_timeout", kafkax.DefaultConfig().Consumer.SessionTimeout)
	v.SetDefault("kafka.consumer.rebalance_timeout", kafkax.DefaultConfig().Consumer.RebalanceTimeout)
	v.SetDefault("kafka.consumer.start_offset", kafkax.DefaultConfig().Consumer.StartOffset)
	v.SetDefault("kafka.consumer.watch_partition_changes", kafkax.DefaultConfig().Consumer.WatchPartitionChanges)
	v.SetDefault("kafka.consumer.max_attempts", kafkax.DefaultConfig().Consumer.MaxAttempts)
	v.SetDefault("kafka.sasl.enable", kafkax.DefaultConfig().SASL.Enable)
	v.SetDefault("kafka.sasl.mechanism", kafkax.DefaultConfig().SASL.Mechanism)
	v.SetDefault("kafka.sasl.username", kafkax.DefaultConfig().SASL.Username)
	v.SetDefault("kafka.sasl.password", kafkax.DefaultConfig().SASL.Password)
	v.SetDefault("kafka.tls.enable", kafkax.DefaultConfig().TLS.Enable)
	v.SetDefault("kafka.tls.insecure_skip_verify", kafkax.DefaultConfig().TLS.InsecureSkipVerify)
	v.SetDefault("kafka.tls.server_name", kafkax.DefaultConfig().TLS.ServerName)
	v.SetDefault("alipay.app_id", alipayx.DefaultConfig().AppID)
	v.SetDefault("alipay.private_key", alipayx.DefaultConfig().PrivateKey)
	v.SetDefault("alipay.alipay_public_key", alipayx.DefaultConfig().AlipayPublicKey)
	v.SetDefault("alipay.production", alipayx.DefaultConfig().Production)
	v.SetDefault("alipay.notify_url", alipayx.DefaultConfig().NotifyURL)
	v.SetDefault("alipay.return_url", alipayx.DefaultConfig().ReturnURL)
	v.SetDefault("alipay.encrypt_key", alipayx.DefaultConfig().EncryptKey)
	v.SetDefault("alipay.app_cert_public_key_path", alipayx.DefaultConfig().AppCertPublicKeyPath)
	v.SetDefault("alipay.alipay_root_cert_path", alipayx.DefaultConfig().AlipayRootCertPath)
	v.SetDefault("alipay.alipay_cert_public_key_path", alipayx.DefaultConfig().AlipayCertPublicKeyPath)
	v.SetDefault("middleware.jwt.secret", middleware.DefaultJWTConfig().Secret)
	v.SetDefault("middleware.jwt.issuer", middleware.DefaultJWTConfig().Issuer)
	v.SetDefault("middleware.jwt.expire_seconds", middleware.DefaultJWTConfig().ExpireSeconds)
	v.SetDefault("middleware.cors.allow_origins", middleware.DefaultCORSConfig().AllowOrigins)
	v.SetDefault("middleware.cors.allow_methods", middleware.DefaultCORSConfig().AllowMethods)
	v.SetDefault("middleware.cors.allow_headers", middleware.DefaultCORSConfig().AllowHeaders)
	v.SetDefault("middleware.cors.expose_headers", middleware.DefaultCORSConfig().ExposeHeaders)
	v.SetDefault("middleware.cors.allow_credentials", middleware.DefaultCORSConfig().AllowCredentials)
	v.SetDefault("log.service", "xiaolanshu")
	v.SetDefault("log.environment", "local")
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "json")
	v.SetDefault("log.file.enabled", true)
	v.SetDefault("log.file.filename", "logs/app.log")
	v.SetDefault("log.file.max_size_mb", 128)
	v.SetDefault("log.file.max_backups", 14)
	v.SetDefault("log.file.max_age_days", 7)
	v.SetDefault("log.file.compress", true)
}

func setNacosDefaults(v *viper.Viper) {
	v.SetDefault("enabled", nacosx.DefaultConfig().Enabled)
	v.SetDefault("server_addr", nacosx.DefaultConfig().ServerAddr)
	v.SetDefault("server_port", nacosx.DefaultConfig().ServerPort)
	v.SetDefault("namespace_id", nacosx.DefaultConfig().NamespaceID)
	v.SetDefault("group", nacosx.DefaultConfig().Group)
	v.SetDefault("data_id", nacosx.DefaultConfig().DataID)
	v.SetDefault("username", nacosx.DefaultConfig().Username)
	v.SetDefault("password", nacosx.DefaultConfig().Password)
	v.SetDefault("timeout_ms", nacosx.DefaultConfig().TimeoutMs)
	v.SetDefault("log_dir", nacosx.DefaultConfig().LogDir)
	v.SetDefault("cache_dir", nacosx.DefaultConfig().CacheDir)
	v.SetDefault("log_level", nacosx.DefaultConfig().LogLevel)
	v.SetDefault("fail_fast", nacosx.DefaultConfig().FailFast)
	v.SetDefault("watch", nacosx.DefaultConfig().Watch)
}
