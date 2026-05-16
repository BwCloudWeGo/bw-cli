package redisx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/pkg/redisx"
)

func TestDefaultConfig(t *testing.T) {
	cfg := redisx.DefaultConfig()

	require.Equal(t, "127.0.0.1:6379", cfg.Addr)
	require.Equal(t, 0, cfg.DB)
	require.Equal(t, 10, cfg.PoolSize)
	require.Equal(t, 5*time.Second, cfg.DialTimeout)
	require.Equal(t, 3*time.Second, cfg.ReadTimeout)
	require.Equal(t, 3*time.Second, cfg.WriteTimeout)
	require.Equal(t, 30*time.Second, cfg.Lock.DefaultTTL)
	require.Equal(t, "xiaolanshu", cfg.Lock.KeyPrefix)
}

func TestNormalizeAppliesFieldDefaults(t *testing.T) {
	cfg := redisx.Config{
		Addr:     "redis:6379",
		Password: "secret",
	}

	normalized := cfg.Normalize()

	require.Equal(t, "redis:6379", normalized.Addr)
	require.Equal(t, "secret", normalized.Password)
	require.Equal(t, 0, normalized.DB)
	require.Equal(t, 10, normalized.PoolSize)
	require.Equal(t, 5*time.Second, normalized.DialTimeout)
	require.Equal(t, 3*time.Second, normalized.ReadTimeout)
	require.Equal(t, 3*time.Second, normalized.WriteTimeout)
	require.Equal(t, 30*time.Second, normalized.Lock.DefaultTTL)
	require.Equal(t, "xiaolanshu", normalized.Lock.KeyPrefix)
}

func TestNewClientUsesNormalizedConfig(t *testing.T) {
	client := redisx.NewClient(redisx.Config{Addr: "redis:6379"})
	defer client.Close()

	options := client.Options()
	require.Equal(t, "redis:6379", options.Addr)
	require.Equal(t, 10, options.PoolSize)
	require.Equal(t, 5*time.Second, options.DialTimeout)
	require.Equal(t, 3*time.Second, options.ReadTimeout)
	require.Equal(t, 3*time.Second, options.WriteTimeout)
}

func TestLockerAcquireReleaseAndConflict(t *testing.T) {
	server := miniredis.RunT(t)
	client := redisx.NewClient(redisx.Config{Addr: server.Addr()})
	defer client.Close()

	locker := redisx.NewLocker(client, redisx.LockConfig{
		KeyPrefix:  "app",
		DefaultTTL: time.Minute,
	})
	ctx := context.Background()

	lock, err := locker.Acquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	require.Equal(t, "app:job:1", lock.Key())
	require.NotEmpty(t, lock.Token())

	_, err = locker.Acquire(ctx, "job:1", time.Minute)
	require.ErrorIs(t, err, redisx.ErrLockNotAcquired)

	require.NoError(t, lock.Release(ctx))
	require.False(t, server.Exists("app:job:1"))
}

func TestLockReleaseDoesNotDeleteAnotherOwner(t *testing.T) {
	server := miniredis.RunT(t)
	client := redisx.NewClient(redisx.Config{Addr: server.Addr()})
	defer client.Close()

	locker := redisx.NewLocker(client, redisx.LockConfig{
		KeyPrefix:  "app",
		DefaultTTL: time.Minute,
	})
	ctx := context.Background()

	lock, err := locker.Acquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	require.NoError(t, client.Set(ctx, "app:job:1", "other-token", time.Minute).Err())

	err = lock.Release(ctx)

	require.ErrorIs(t, err, redisx.ErrLockNotHeld)
	value, err := server.Get("app:job:1")
	require.NoError(t, err)
	require.Equal(t, "other-token", value)
}

func TestLockRefreshExtendsOnlyOwnedLock(t *testing.T) {
	server := miniredis.RunT(t)
	client := redisx.NewClient(redisx.Config{Addr: server.Addr()})
	defer client.Close()

	locker := redisx.NewLocker(client, redisx.LockConfig{
		KeyPrefix:  "app",
		DefaultTTL: time.Minute,
	})
	ctx := context.Background()

	lock, err := locker.Acquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	server.FastForward(30 * time.Second)

	require.NoError(t, lock.Refresh(ctx, 2*time.Minute))
	require.Greater(t, server.TTL("app:job:1"), time.Minute)

	require.NoError(t, client.Set(ctx, "app:job:1", "other-token", time.Minute).Err())
	err = lock.Refresh(ctx, time.Minute)
	require.ErrorIs(t, err, redisx.ErrLockNotHeld)
}

func TestLockerTryAcquireReturnsFalseWhenHeld(t *testing.T) {
	server := miniredis.RunT(t)
	client := redisx.NewClient(redisx.Config{Addr: server.Addr()})
	defer client.Close()

	locker := redisx.NewLocker(client, redisx.LockConfig{
		KeyPrefix:  "app",
		DefaultTTL: time.Minute,
	})
	ctx := context.Background()

	lock, acquired, err := locker.TryAcquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NotNil(t, lock)

	lock, acquired, err = locker.TryAcquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	require.False(t, acquired)
	require.Nil(t, lock)
}

func TestLockReleaseIsIdempotentForAlreadyReleasedLock(t *testing.T) {
	server := miniredis.RunT(t)
	client := redisx.NewClient(redisx.Config{Addr: server.Addr()})
	defer client.Close()

	locker := redisx.NewLocker(client, redisx.LockConfig{
		KeyPrefix:  "app",
		DefaultTTL: time.Minute,
	})
	ctx := context.Background()

	lock, err := locker.Acquire(ctx, "job:1", time.Minute)
	require.NoError(t, err)
	require.NoError(t, lock.Release(ctx))
	require.True(t, errors.Is(lock.Release(ctx), redisx.ErrLockNotHeld))
}
