package mongox

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Config 是 MongoDB 客户端的连接配置。
// 它只描述公共连接能力，不绑定任何具体业务集合；业务集合操作应放在各服务 repo 层。
type Config struct {
	URI                    string        `mapstructure:"uri" yaml:"uri"`
	Username               string        `mapstructure:"username" yaml:"username"`
	Password               string        `mapstructure:"password" yaml:"password"`
	Database               string        `mapstructure:"database" yaml:"database"`
	AppName                string        `mapstructure:"app_name" yaml:"app_name"`
	MinPoolSize            uint64        `mapstructure:"min_pool_size" yaml:"min_pool_size"`
	MaxPoolSize            uint64        `mapstructure:"max_pool_size" yaml:"max_pool_size"`
	ConnectTimeout         time.Duration `mapstructure:"connect_timeout" yaml:"connect_timeout"`
	ServerSelectionTimeout time.Duration `mapstructure:"server_selection_timeout" yaml:"server_selection_timeout"`
}

// DefaultConfig 返回本地开发环境可直接使用的 MongoDB 默认配置。
// 生产环境建议通过配置文件或 APP_* 环境变量覆盖 URI、账号、密码和连接池参数。
func DefaultConfig() Config {
	return Config{
		URI:                    "mongodb://127.0.0.1:27017",
		Username:               "",
		Password:               "",
		Database:               "xiaolanshu",
		AppName:                "bw-cli",
		MinPoolSize:            0,
		MaxPoolSize:            100,
		ConnectTimeout:         10 * time.Second,
		ServerSelectionTimeout: 5 * time.Second,
	}
}

// NewClient 根据配置创建 MongoDB 客户端。
// 该方法只负责构建客户端，不主动发起网络探测；调用方可在启动阶段显式调用 Ping 做健康检查。
func NewClient(cfg Config) (*mongo.Client, error) {
	defaults := DefaultConfig()
	if cfg.URI == "" {
		cfg.URI = defaults.URI
	}
	if cfg.Database == "" {
		cfg.Database = defaults.Database
	}
	if cfg.AppName == "" {
		cfg.AppName = defaults.AppName
	}
	if cfg.MaxPoolSize == 0 {
		cfg.MaxPoolSize = defaults.MaxPoolSize
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = defaults.ConnectTimeout
	}
	if cfg.ServerSelectionTimeout == 0 {
		cfg.ServerSelectionTimeout = defaults.ServerSelectionTimeout
	}

	return mongo.Connect(clientOptions(cfg))
}

func clientOptions(cfg Config) *options.ClientOptions {
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetAppName(cfg.AppName).
		SetMinPoolSize(cfg.MinPoolSize).
		SetMaxPoolSize(cfg.MaxPoolSize).
		SetConnectTimeout(cfg.ConnectTimeout).
		SetServerSelectionTimeout(cfg.ServerSelectionTimeout)
	if cfg.Username != "" || cfg.Password != "" {
		auth := options.Credential{Username: cfg.Username, Password: cfg.Password, PasswordSet: cfg.Password != ""}
		clientOptions.SetAuth(auth)
	}
	return clientOptions
}

// Database 根据已有客户端返回指定数据库句柄。
// 数据库名称通常来自配置文件的 mongodb.database，避免在业务代码中写死。
func Database(client *mongo.Client, name string) *mongo.Database {
	return client.Database(name)
}

// Ping 检查 MongoDB 客户端是否能连接到主节点或可读副本节点。
// 服务启动时建议调用一次，便于提前暴露配置错误或网络问题。
func Ping(ctx context.Context, client *mongo.Client) error {
	return client.Ping(ctx, readpref.PrimaryPreferred())
}
