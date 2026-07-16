package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestInitRedis_MissingRedisURL(t *testing.T) {
	os.Unsetenv("REDIS_URL")

	err := InitRedis()
	if err == nil {
		t.Fatal("expected error when REDIS_URL is not set")
	}
	if !strings.Contains(err.Error(), "REDIS_URL environment variable is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInitRedis_CustomAddr(t *testing.T) {
	os.Setenv("REDIS_URL", "localhost:19999")
	defer os.Unsetenv("REDIS_URL")

	err := InitRedis()
	if err == nil {
		t.Fatal("expected error when no Redis is available at custom addr")
	}
}

func TestInitRedis_PingError(t *testing.T) {
	origNew := redisNewClient
	origPing := redisPing
	origRedis := Redis
	defer func() {
		redisNewClient = origNew
		redisPing = origPing
		Redis = origRedis
	}()

	os.Setenv("REDIS_URL", "localhost:19999")
	defer os.Unsetenv("REDIS_URL")

	redisPing = func(ctx context.Context, client *redis.Client) error {
		return fmt.Errorf("connection refused")
	}

	err := InitRedis()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ping redis") {
		t.Fatalf("expected 'ping redis' error, got: %v", err)
	}
}

func TestInitRedis_Success(t *testing.T) {
	origNew := redisNewClient
	origPing := redisPing
	origRedis := Redis
	defer func() {
		redisNewClient = origNew
		redisPing = origPing
		if Redis != nil {
			Redis.Close()
		}
		Redis = origRedis
	}()

	os.Setenv("REDIS_URL", "localhost:6379")
	defer os.Unsetenv("REDIS_URL")

	var capturedClient *redis.Client
	redisNewClient = func(opts *redis.Options) *redis.Client {
		c := redis.NewClient(opts)
		capturedClient = c
		return c
	}
	redisPing = func(ctx context.Context, client *redis.Client) error {
		return nil // pretend ping succeeded
	}

	err := InitRedis()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if Redis == nil {
		t.Fatal("Redis should be set after successful init")
	}
	if Redis != capturedClient {
		t.Fatal("Redis should be the client returned by NewClient")
	}
}

func TestCloseRedis_NilClient(t *testing.T) {
	origRedis := Redis
	defer func() { Redis = origRedis }()

	Redis = nil
	CloseRedis() // should not panic
}

func TestCloseRedis_NonNilClient(t *testing.T) {
	origRedis := Redis
	defer func() { Redis = origRedis }()

	// Create a real client (won't actually connect until used)
	Redis = redis.NewClient(&redis.Options{Addr: "localhost:19999"})
	CloseRedis() // should close without panic
}
