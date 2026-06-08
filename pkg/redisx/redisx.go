package redisx

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// Config 控制 Redis 客户端连接和连接池设置。
type Config struct {
	Addr         string        `mapstructure:"addr" yaml:"addr"`
	Username     string        `mapstructure:"username" yaml:"username"`
	Password     string        `mapstructure:"password" yaml:"password"`
	DB           int           `mapstructure:"db" yaml:"db"`
	PoolSize     int           `mapstructure:"pool_size" yaml:"pool_size"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout" yaml:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout" yaml:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout" yaml:"write_timeout"`
	Lock         LockConfig    `mapstructure:"lock" yaml:"lock"`
}

// LockConfig 控制 Redis 分布式锁行为。
type LockConfig struct {
	KeyPrefix  string        `mapstructure:"key_prefix" yaml:"key_prefix"`
	DefaultTTL time.Duration `mapstructure:"default_ttl" yaml:"default_ttl"`
}

// DefaultConfig 返回本地开发可用的 Redis 默认配置。
func DefaultConfig() Config {
	return Config{
		Addr:         "127.0.0.1:6379",
		DB:           0,
		PoolSize:     10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		Lock: LockConfig{
			KeyPrefix:  "xiaolanshu",
			DefaultTTL: 30 * time.Second,
		},
	}
}

// Normalize 在保留调用方配置的同时补齐默认值。
func (cfg Config) Normalize() Config {
	defaults := DefaultConfig()
	if cfg.Addr == "" {
		cfg.Addr = defaults.Addr
	}
	if cfg.PoolSize == 0 {
		cfg.PoolSize = defaults.PoolSize
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = defaults.DialTimeout
	}
	if cfg.ReadTimeout == 0 {
		cfg.ReadTimeout = defaults.ReadTimeout
	}
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaults.WriteTimeout
	}
	if cfg.Lock.KeyPrefix == "" {
		cfg.Lock.KeyPrefix = defaults.Lock.KeyPrefix
	}
	if cfg.Lock.DefaultTTL == 0 {
		cfg.Lock.DefaultTTL = defaults.Lock.DefaultTTL
	}
	return cfg
}

// NewClient 根据配置创建 go-redis 客户端。
func NewClient(cfg Config) *redis.Client {
	cfg = cfg.Normalize()
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

// Ping 检查 Redis 客户端是否能连接到配置的服务端。
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

var (
	// ErrLockNotAcquired 表示锁已被其他持有者占用。
	ErrLockNotAcquired = errors.New("redis lock not acquired")
	// ErrLockNotHeld 表示当前锁 token 已不再持有该 key。
	ErrLockNotHeld = errors.New("redis lock not held")
)

const (
	releaseScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0
`
	refreshScript = `
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
return 0
`
)

// Locker 创建并管理 Redis 分布式锁。
type Locker struct {
	client *redis.Client
	cfg    LockConfig
}

// NewLocker 创建分布式锁管理器。
func NewLocker(client *redis.Client, cfg LockConfig) *Locker {
	defaults := DefaultConfig().Lock
	if cfg.KeyPrefix == "" {
		cfg.KeyPrefix = defaults.KeyPrefix
	}
	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = defaults.DefaultTTL
	}
	return &Locker{client: client, cfg: cfg}
}

// Acquire 获取锁；若锁已被持有则返回 ErrLockNotAcquired。
func (l *Locker) Acquire(ctx context.Context, key string, ttl time.Duration) (*Lock, error) {
	lock, acquired, err := l.TryAcquire(ctx, key, ttl)
	if err != nil {
		return nil, err
	}
	if !acquired {
		return nil, ErrLockNotAcquired
	}
	return lock, nil
}

// TryAcquire 尝试获取锁，并返回是否成功。
func (l *Locker) TryAcquire(ctx context.Context, key string, ttl time.Duration) (*Lock, bool, error) {
	if l == nil || l.client == nil {
		return nil, false, errors.New("redis locker client is nil")
	}
	if ttl == 0 {
		ttl = l.cfg.DefaultTTL
	}
	if ttl <= 0 {
		return nil, false, fmt.Errorf("redis lock ttl must be positive: %s", ttl)
	}
	token, err := newToken()
	if err != nil {
		return nil, false, err
	}
	fullKey := l.fullKey(key)
	acquired, err := l.client.SetNX(ctx, fullKey, token, ttl).Result()
	if err != nil {
		return nil, false, err
	}
	if !acquired {
		return nil, false, nil
	}
	return &Lock{
		client: l.client,
		key:    fullKey,
		token:  token,
	}, true, nil
}

func (l *Locker) fullKey(key string) string {
	key = strings.TrimSpace(key)
	prefix := strings.Trim(l.cfg.KeyPrefix, ":")
	if prefix == "" {
		return key
	}
	return prefix + ":" + key
}

// Lock 表示已获取的 Redis 分布式锁。
type Lock struct {
	client *redis.Client
	key    string
	token  string
}

// Key 返回锁使用的 Redis 键。
func (l *Lock) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// Token 返回当前持有者 token，便于诊断和测试。
func (l *Lock) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

// Release 仅在 token 仍持有锁时删除该锁。
func (l *Lock) Release(ctx context.Context) error {
	if l == nil || l.client == nil {
		return ErrLockNotHeld
	}
	result, err := l.client.Eval(ctx, releaseScript, []string{l.key}, l.token).Int64()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

// Refresh 仅在 token 仍持有锁时延长租约。
func (l *Lock) Refresh(ctx context.Context, ttl time.Duration) error {
	if l == nil || l.client == nil {
		return ErrLockNotHeld
	}
	if ttl <= 0 {
		return fmt.Errorf("redis lock ttl must be positive: %s", ttl)
	}
	result, err := l.client.Eval(ctx, refreshScript, []string{l.key}, l.token, ttl.Milliseconds()).Int64()
	if err != nil {
		return err
	}
	if result == 0 {
		return ErrLockNotHeld
	}
	return nil
}

func newToken() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes[:]), nil
}
