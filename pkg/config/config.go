package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"

	"github.com/BwCloudWeGo/bw-cli/pkg/esx"
	"github.com/BwCloudWeGo/bw-cli/pkg/kafkax"
	"github.com/BwCloudWeGo/bw-cli/pkg/logger"
	"github.com/BwCloudWeGo/bw-cli/pkg/middleware"
	"github.com/BwCloudWeGo/bw-cli/pkg/mongox"
	"github.com/BwCloudWeGo/bw-cli/pkg/mysqlx"
	"github.com/BwCloudWeGo/bw-cli/pkg/postgresx"
	"github.com/BwCloudWeGo/bw-cli/pkg/redisx"
)

// AppConfig contains project and service identity values shared by all processes.
type AppConfig struct {
	Name               string `mapstructure:"name" yaml:"name"`
	Env                string `mapstructure:"env" yaml:"env"`
	GatewayServiceName string `mapstructure:"gateway_service_name" yaml:"gateway_service_name"`
	UserServiceName    string `mapstructure:"user_service_name" yaml:"user_service_name"`
	NoteServiceName    string `mapstructure:"note_service_name" yaml:"note_service_name"`
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
	Host       string `mapstructure:"host" yaml:"host"`
	UserPort   int    `mapstructure:"user_port" yaml:"user_port"`
	NotePort   int    `mapstructure:"note_port" yaml:"note_port"`
	UserTarget string `mapstructure:"user_target" yaml:"user_target"`
	NoteTarget string `mapstructure:"note_target" yaml:"note_target"`
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

// MongoDBConfig contains MongoDB client, database and pool settings loaded from YAML/env.
type MongoDBConfig struct {
	URI                           string `mapstructure:"uri" yaml:"uri"`
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

// MiddlewareConfig groups HTTP middleware configuration loaded from YAML/env.
type MiddlewareConfig struct {
	CORS middleware.CORSConfig `mapstructure:"cors" yaml:"cors"`
	JWT  middleware.JWTConfig  `mapstructure:"jwt" yaml:"jwt"`
}

// Config is the root application configuration loaded by each process.
type Config struct {
	App           AppConfig        `mapstructure:"app" yaml:"app"`
	HTTP          HTTPConfig       `mapstructure:"http" yaml:"http"`
	GRPC          GRPCConfig       `mapstructure:"grpc" yaml:"grpc"`
	Database      DatabaseConfig   `mapstructure:"database" yaml:"database"`
	MySQL         MySQLConfig      `mapstructure:"mysql" yaml:"mysql"`
	PostgreSQL    PostgreSQLConfig `mapstructure:"postgresql" yaml:"postgresql"`
	MongoDB       MongoDBConfig    `mapstructure:"mongodb" yaml:"mongodb"`
	Redis         redisx.Config    `mapstructure:"redis" yaml:"redis"`
	Elasticsearch esx.Config       `mapstructure:"elasticsearch" yaml:"elasticsearch"`
	Kafka         kafkax.Config    `mapstructure:"kafka" yaml:"kafka"`
	Middleware    MiddlewareConfig `mapstructure:"middleware" yaml:"middleware"`
	Log           logger.Config    `mapstructure:"log" yaml:"log"`
}

// Load reads YAML configuration and applies APP_* environment overrides.
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

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
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("app.name", "xiaolanshu")
	v.SetDefault("app.env", "local")
	v.SetDefault("app.gateway_service_name", "gateway")
	v.SetDefault("app.user_service_name", "user-service")
	v.SetDefault("app.note_service_name", "note-service")
	v.SetDefault("http.host", "0.0.0.0")
	v.SetDefault("http.port", 8080)
	v.SetDefault("http.read_timeout_seconds", 5)
	v.SetDefault("http.write_timeout_seconds", 10)
	v.SetDefault("grpc.host", "0.0.0.0")
	v.SetDefault("grpc.user_port", 9001)
	v.SetDefault("grpc.note_port", 9002)
	v.SetDefault("grpc.user_target", "127.0.0.1:9001")
	v.SetDefault("grpc.note_target", "127.0.0.1:9002")
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
	v.SetDefault("mongodb.database", mongox.DefaultConfig().Database)
	v.SetDefault("mongodb.app_name", mongox.DefaultConfig().AppName)
	v.SetDefault("mongodb.min_pool_size", mongox.DefaultConfig().MinPoolSize)
	v.SetDefault("mongodb.max_pool_size", mongox.DefaultConfig().MaxPoolSize)
	v.SetDefault("mongodb.connect_timeout_seconds", int(mongox.DefaultConfig().ConnectTimeout/time.Second))
	v.SetDefault("mongodb.server_selection_timeout_seconds", int(mongox.DefaultConfig().ServerSelectionTimeout/time.Second))
	v.SetDefault("redis.addr", redisx.DefaultConfig().Addr)
	v.SetDefault("redis.db", redisx.DefaultConfig().DB)
	v.SetDefault("redis.pool_size", redisx.DefaultConfig().PoolSize)
	v.SetDefault("elasticsearch.addresses", esx.DefaultConfig().Addresses)
	v.SetDefault("kafka.brokers", kafkax.DefaultConfig().Brokers)
	v.SetDefault("kafka.topic", kafkax.DefaultConfig().Topic)
	v.SetDefault("kafka.group_id", kafkax.DefaultConfig().GroupID)
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
