package redisx

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config controls Redis client connection and pool settings.
type Config struct {
	Addr         string        `mapstructure:"addr" yaml:"addr"`
	Username     string        `mapstructure:"username" yaml:"username"`
	Password     string        `mapstructure:"password" yaml:"password"`
	DB           int           `mapstructure:"db" yaml:"db"`
	PoolSize     int           `mapstructure:"pool_size" yaml:"pool_size"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
}

// DefaultConfig returns local-development Redis defaults.
func DefaultConfig() Config {
	return Config{
		Addr:         "127.0.0.1:6379",
		DB:           0,
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewClient creates a go-redis client from configuration.
func NewClient(cfg Config) *redis.Client {
	if cfg.Addr == "" {
		cfg = DefaultConfig()
	}
	return redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}

// Ping checks that the Redis client can reach the configured server.
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}
