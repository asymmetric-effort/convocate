//go:build integration

package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
)

const defaultTestRedisAddr = "redis.data-layer.svc:6379"

func testRedisAddr() string {
	if v := os.Getenv("TEST_REDIS_URL"); v != "" {
		return v
	}
	return defaultTestRedisAddr
}

func setupIntegrationRedis(t *testing.T) {
	t.Helper()
	os.Setenv("REDIS_URL", testRedisAddr())
	t.Cleanup(func() {
		os.Unsetenv("REDIS_URL")
	})
}

func TestIntegration_InitRedis(t *testing.T) {
	setupIntegrationRedis(t)

	err := InitRedis()
	if err != nil {
		t.Skipf("cannot connect to Redis (skipping): %v", err)
	}
	t.Cleanup(func() {
		CloseRedis()
		Redis = nil
	})

	if Redis == nil {
		t.Fatal("Redis client should not be nil after successful init")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pong, err := Redis.Ping(ctx).Result()
	if err != nil {
		t.Fatalf("ping after init: %v", err)
	}
	if pong != "PONG" {
		t.Errorf("ping result = %q, want PONG", pong)
	}
	t.Log("InitRedis connected successfully")
}

func TestIntegration_Redis_SetGetDel(t *testing.T) {
	setupIntegrationRedis(t)

	err := InitRedis()
	if err != nil {
		t.Skipf("cannot connect to Redis (skipping): %v", err)
	}
	t.Cleanup(func() {
		CloseRedis()
		Redis = nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("convocate:integration-test:%d", time.Now().UnixNano())
	value := "test-value-12345"

	// SET
	err = Redis.Set(ctx, key, value, 30*time.Second).Err()
	if err != nil {
		t.Fatalf("SET: %v", err)
	}
	t.Logf("SET %s = %s", key, value)

	// GET
	got, err := Redis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	if got != value {
		t.Errorf("GET = %q, want %q", got, value)
	}
	t.Log("GET returned correct value")

	// DEL
	deleted, err := Redis.Del(ctx, key).Result()
	if err != nil {
		t.Fatalf("DEL: %v", err)
	}
	if deleted != 1 {
		t.Errorf("DEL count = %d, want 1", deleted)
	}

	// Verify key is gone
	_, err = Redis.Get(ctx, key).Result()
	if err == nil {
		t.Fatal("expected error after DEL, got nil")
	}
	t.Log("DEL verified — key not found after deletion")
}

func TestIntegration_Redis_HashOps(t *testing.T) {
	setupIntegrationRedis(t)

	err := InitRedis()
	if err != nil {
		t.Skipf("cannot connect to Redis (skipping): %v", err)
	}
	t.Cleanup(func() {
		CloseRedis()
		Redis = nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	hashKey := fmt.Sprintf("convocate:integration-test:hash:%d", time.Now().UnixNano())
	t.Cleanup(func() {
		Redis.Del(context.Background(), hashKey)
	})

	// HSET
	err = Redis.HSet(ctx, hashKey, map[string]interface{}{
		"field1": "value1",
		"field2": "value2",
	}).Err()
	if err != nil {
		t.Fatalf("HSET: %v", err)
	}

	// HGET
	got, err := Redis.HGet(ctx, hashKey, "field1").Result()
	if err != nil {
		t.Fatalf("HGET: %v", err)
	}
	if got != "value1" {
		t.Errorf("HGET field1 = %q, want %q", got, "value1")
	}

	// HGETALL
	all, err := Redis.HGetAll(ctx, hashKey).Result()
	if err != nil {
		t.Fatalf("HGETALL: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("HGETALL returned %d fields, want 2", len(all))
	}
	t.Log("hash operations verified")
}

func TestIntegration_Redis_Expiry(t *testing.T) {
	setupIntegrationRedis(t)

	err := InitRedis()
	if err != nil {
		t.Skipf("cannot connect to Redis (skipping): %v", err)
	}
	t.Cleanup(func() {
		CloseRedis()
		Redis = nil
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("convocate:integration-test:expiry:%d", time.Now().UnixNano())

	err = Redis.Set(ctx, key, "expires-soon", 1*time.Second).Err()
	if err != nil {
		t.Fatalf("SET with expiry: %v", err)
	}

	// Key should exist immediately
	got, err := Redis.Get(ctx, key).Result()
	if err != nil {
		t.Fatalf("GET before expiry: %v", err)
	}
	if got != "expires-soon" {
		t.Errorf("GET = %q, want %q", got, "expires-soon")
	}

	// Wait for expiry
	time.Sleep(1100 * time.Millisecond)

	_, err = Redis.Get(ctx, key).Result()
	if err == nil {
		t.Error("expected key to have expired")
	}
	t.Log("TTL expiry verified")
}

func TestIntegration_CloseRedis(t *testing.T) {
	setupIntegrationRedis(t)

	err := InitRedis()
	if err != nil {
		t.Skipf("cannot connect to Redis (skipping): %v", err)
	}

	if Redis == nil {
		t.Fatal("Redis should not be nil")
	}

	CloseRedis()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = Redis.Ping(ctx).Err()
	if err == nil {
		t.Error("expected Ping to fail after CloseRedis")
	} else {
		t.Logf("Ping after close returned expected error: %v", err)
	}
	Redis = nil
}

func TestIntegration_Redis_WrongAddr(t *testing.T) {
	os.Setenv("REDIS_URL", "localhost:59999")
	defer os.Unsetenv("REDIS_URL")

	err := InitRedis()
	if err == nil {
		CloseRedis()
		Redis = nil
		t.Fatal("expected error for wrong Redis address")
	}
	t.Logf("wrong address error: %v", err)
}
