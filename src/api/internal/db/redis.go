package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

var Redis *redis.Client

func defaultRedisNewClient(opts *redis.Options) *redis.Client {
	return redis.NewClient(opts)
}

func defaultRedisPing(ctx context.Context, client *redis.Client) error {
	return client.Ping(ctx).Err()
}

// redisNewClient creates a Redis client; tests replace it to avoid real connections.
var redisNewClient = defaultRedisNewClient

// redisPing pings the Redis client; tests replace it to simulate ping failures.
var redisPing = defaultRedisPing

func InitRedis() error {
	addr := os.Getenv("REDIS_URL")
	if addr == "" {
		return fmt.Errorf("REDIS_URL environment variable is required")
	}

	Redis = redisNewClient(&redis.Options{
		Addr:         addr,
		DB:           0,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     20,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := redisPing(ctx, Redis); err != nil {
		return fmt.Errorf("ping redis: %w", err)
	}

	return nil
}

func CloseRedis() {
	if Redis != nil {
		Redis.Close()
	}
}
