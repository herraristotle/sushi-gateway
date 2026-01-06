package gateway

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// GlobalRedisClient is the shared Redis client for rate limiting and caching
var GlobalRedisClient *redis.Client

// RedisConfig holds Redis connection configuration
type RedisConfig struct {
	Addr     string
	Password string
	DB       int
}

// InitRedisClient initializes the global Redis client
// Redis is REQUIRED for distributed rate limiting and caching
func InitRedisClient(cfg RedisConfig) error {
	GlobalRedisClient = redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     100,
		MinIdleConns: 10,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := GlobalRedisClient.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %w", cfg.Addr, err)
	}

	slog.Info("Redis connected successfully", "addr", cfg.Addr)
	return nil
}

// CloseRedisClient closes the Redis connection
func CloseRedisClient() error {
	if GlobalRedisClient != nil {
		return GlobalRedisClient.Close()
	}
	return nil
}

// RateLimitKey generates a rate limit key for Redis
func RateLimitKey(identifier string, window string) string {
	return fmt.Sprintf("ratelimit:%s:%s", identifier, window)
}

// CacheKey generates a cache key for Redis
func CacheKey(method, path string) string {
	return fmt.Sprintf("cache:%s:%s", method, path)
}
