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
	Lock         LockConfig    `mapstructure:"lock" yaml:"lock"`
}

// LockConfig controls Redis distributed lock behavior.
type LockConfig struct {
	KeyPrefix  string        `mapstructure:"key_prefix" yaml:"key_prefix"`
	DefaultTTL time.Duration `mapstructure:"default_ttl" yaml:"default_ttl"`
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
		Lock: LockConfig{
			KeyPrefix:  "xiaolanshu",
			DefaultTTL: 30 * time.Second,
		},
	}
}

// Normalize applies defaults while preserving caller-provided fields.
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

// NewClient creates a go-redis client from configuration.
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

// Ping checks that the Redis client can reach the configured server.
func Ping(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

var (
	// ErrLockNotAcquired indicates that another owner already holds the lock.
	ErrLockNotAcquired = errors.New("redis lock not acquired")
	// ErrLockNotHeld indicates that this lock token no longer owns the key.
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

// Locker creates and manages Redis distributed locks.
type Locker struct {
	client *redis.Client
	cfg    LockConfig
}

// NewLocker creates a distributed lock manager.
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

// Acquire obtains a lock or returns ErrLockNotAcquired when it is already held.
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

// TryAcquire obtains a lock and reports whether acquisition succeeded.
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

// Lock is an acquired Redis distributed lock.
type Lock struct {
	client *redis.Client
	key    string
	token  string
}

// Key returns the Redis key used by the lock.
func (l *Lock) Key() string {
	if l == nil {
		return ""
	}
	return l.key
}

// Token returns this owner token. It is useful for diagnostics and tests.
func (l *Lock) Token() string {
	if l == nil {
		return ""
	}
	return l.token
}

// Release deletes the lock only if the token still owns it.
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

// Refresh extends the lock lease only if the token still owns it.
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
