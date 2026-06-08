package nacosx

import (
	"errors"
	"strings"

	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Config 控制可选的 Nacos 配置中心集成。
type Config struct {
	Enabled     bool   `mapstructure:"enabled" yaml:"enabled"`
	ServerAddr  string `mapstructure:"server_addr" yaml:"server_addr"`
	ServerPort  uint64 `mapstructure:"server_port" yaml:"server_port"`
	NamespaceID string `mapstructure:"namespace_id" yaml:"namespace_id"`
	Group       string `mapstructure:"group" yaml:"group"`
	DataID      string `mapstructure:"data_id" yaml:"data_id"`
	Username    string `mapstructure:"username" yaml:"username"`
	Password    string `mapstructure:"password" yaml:"password"`
	TimeoutMs   uint64 `mapstructure:"timeout_ms" yaml:"timeout_ms"`
	LogDir      string `mapstructure:"log_dir" yaml:"log_dir"`
	CacheDir    string `mapstructure:"cache_dir" yaml:"cache_dir"`
	LogLevel    string `mapstructure:"log_level" yaml:"log_level"`
	FailFast    bool   `mapstructure:"fail_fast" yaml:"fail_fast"`
	Watch       bool   `mapstructure:"watch" yaml:"watch"`
}

// DefaultConfig 返回保持 Nacos 关闭的本地开发默认配置。
func DefaultConfig() Config {
	return Config{
		Enabled:    false,
		ServerAddr: "127.0.0.1",
		ServerPort: 8848,
		Group:      "DEFAULT_GROUP",
		DataID:     "xiaolanshu.yaml",
		TimeoutMs:  5000,
		LogDir:     "logs/nacos",
		CacheDir:   "data/nacos/cache",
		LogLevel:   "info",
	}
}

// WithDefaults 填充可选空字段，但不会强制开启 Nacos。
func WithDefaults(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.ServerAddr == "" {
		cfg.ServerAddr = defaults.ServerAddr
	}
	if cfg.ServerPort == 0 {
		cfg.ServerPort = defaults.ServerPort
	}
	if cfg.Group == "" {
		cfg.Group = defaults.Group
	}
	if cfg.DataID == "" {
		cfg.DataID = defaults.DataID
	}
	if cfg.TimeoutMs == 0 {
		cfg.TimeoutMs = defaults.TimeoutMs
	}
	if cfg.LogDir == "" {
		cfg.LogDir = defaults.LogDir
	}
	if cfg.CacheDir == "" {
		cfg.CacheDir = defaults.CacheDir
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = defaults.LogLevel
	}
	return cfg
}

// LoadConfig 从 Nacos 拉取一份 YAML 配置文档。
func LoadConfig(cfg Config) (string, error) {
	cfg = WithDefaults(cfg)
	if !cfg.Enabled {
		return "", nil
	}
	if strings.TrimSpace(cfg.DataID) == "" {
		return "", errors.New("nacos data_id is required")
	}
	if strings.TrimSpace(cfg.Group) == "" {
		return "", errors.New("nacos group is required")
	}

	client, err := NewConfigClient(cfg)
	if err != nil {
		return "", err
	}
	return client.GetConfig(vo.ConfigParam{
		DataId: cfg.DataID,
		Group:  cfg.Group,
	})
}

// NewConfigClient 根据框架配置创建官方 Nacos 配置客户端。
func NewConfigClient(cfg Config) (config_client.IConfigClient, error) {
	cfg = WithDefaults(cfg)
	return clients.NewConfigClient(
		vo.NacosClientParam{
			ClientConfig: &constant.ClientConfig{
				NamespaceId:         cfg.NamespaceID,
				TimeoutMs:           cfg.TimeoutMs,
				Username:            cfg.Username,
				Password:            cfg.Password,
				LogDir:              cfg.LogDir,
				CacheDir:            cfg.CacheDir,
				LogLevel:            cfg.LogLevel,
				NotLoadCacheAtStart: true,
			},
			ServerConfigs: []constant.ServerConfig{
				{
					IpAddr: cfg.ServerAddr,
					Port:   cfg.ServerPort,
				},
			},
		},
	)
}
