package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	// ErrCacheMiss indicates the requested key does not exist or has expired.
	ErrCacheMiss = errors.New("cache: key not found")
)

// CacheService defines a standard key-value caching contract for services.
type CacheService interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
	Ping(ctx context.Context) error
}

// RedisCache implements CacheService backed by a real Redis instance.
type RedisCache struct {
	redisClient *redis.Client
}

var _ CacheService = (*RedisCache)(nil)

// NewRedisCache creates a new RedisCache instance from an active redis.Client.
func NewRedisCache(redisClient *redis.Client) *RedisCache {
	return &RedisCache{
		redisClient: redisClient,
	}
}

func (cache *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	bytes, err := cache.redisClient.Get(ctx, key).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("redis get error: %w", err)
	}
	return bytes, nil
}

func (cache *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := cache.redisClient.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	return nil
}

func (cache *RedisCache) Delete(ctx context.Context, key string) error {
	if err := cache.redisClient.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete error: %w", err)
	}
	return nil
}

func (cache *RedisCache) Ping(ctx context.Context) error {
	return cache.redisClient.Ping(ctx).Err()
}

// NoOpCache is a fallback cache implementation used when Redis is unavailable or disabled.
type NoOpCache struct{}

var _ CacheService = (*NoOpCache)(nil)

func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

func (cache *NoOpCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, ErrCacheMiss
}

func (cache *NoOpCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (cache *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (cache *NoOpCache) Ping(ctx context.Context) error {
	return nil
}
