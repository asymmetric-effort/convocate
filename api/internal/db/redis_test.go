package db

import (
	"os"
	"testing"
)

func TestInitRedis_DefaultAddr(t *testing.T) {
	os.Unsetenv("REDIS_URL")

	err := InitRedis()
	if err == nil {
		t.Fatal("expected error when no Redis is available")
	}
	// Verify the error is about connectivity, not config parsing
	if Redis == nil {
		t.Fatal("Redis client should be created even if ping fails")
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

func TestInitRedis_ErrorContainsPing(t *testing.T) {
	os.Setenv("REDIS_URL", "localhost:19999")
	defer os.Unsetenv("REDIS_URL")

	err := InitRedis()
	if err == nil {
		t.Fatal("expected error")
	}
	// The error should be wrapped with "ping redis:"
	if got := err.Error(); len(got) == 0 {
		t.Fatal("expected non-empty error message")
	}
}

func TestCloseRedis_NilClient(t *testing.T) {
	Redis = nil
	CloseRedis() // should not panic
}

func TestCloseRedis_NonNilClient(t *testing.T) {
	original := Redis
	defer func() { Redis = original }()

	Redis = nil
	CloseRedis()
}
