package db

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestInitPostgres_DefaultDSN(t *testing.T) {
	os.Unsetenv("DATABASE_URL")

	// Use real constructor — will fail at connect because no PG is running.
	origNew := pgNewWithConfig
	origPing := pgPing
	defer func() {
		pgNewWithConfig = origNew
		pgPing = origPing
	}()

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error when no PG is available")
	}
}

func TestInitPostgres_CustomDSN(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://user:pass@localhost:9999/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error when no PG is available")
	}
}

func TestInitPostgres_InvalidDSN(t *testing.T) {
	os.Setenv("DATABASE_URL", "not-a-valid-dsn://:::")
	defer os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
	if !strings.Contains(err.Error(), "parse database url") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestInitPostgres_ConnectError(t *testing.T) {
	origNew := pgNewWithConfig
	origPing := pgPing
	defer func() {
		pgNewWithConfig = origNew
		pgPing = origPing
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	pgNewWithConfig = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, fmt.Errorf("connection refused")
	}

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "connect to postgres") {
		t.Fatalf("expected connect error, got: %v", err)
	}
}

func TestInitPostgres_PingError(t *testing.T) {
	origNew := pgNewWithConfig
	origPing := pgPing
	origPool := Pool
	defer func() {
		pgNewWithConfig = origNew
		pgPing = origPing
		Pool = origPool
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	// NewWithConfig succeeds but returns a pool that will fail ping
	pgNewWithConfig = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		// Return a real pool object (won't actually connect until used)
		// We mock ping separately
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			// If we can't even create the pool config, create a minimal one
			// that still returns a valid pool pointer
			minCfg, _ := pgxpool.ParseConfig("postgres://localhost:5432/test")
			if minCfg != nil {
				minCfg.MaxConns = 1
				p, e := pgxpool.NewWithConfig(ctx, minCfg)
				if e == nil {
					return p, nil
				}
			}
			// Fallback: we need some non-nil pool. Use the original.
			return origNew(ctx, cfg)
		}
		return pool, nil
	}

	pgPing = func(ctx context.Context, pool *pgxpool.Pool) error {
		return fmt.Errorf("ping timeout")
	}

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected ping error")
	}
	if !strings.Contains(err.Error(), "ping postgres") {
		t.Fatalf("expected 'ping postgres' error, got: %v", err)
	}
}

func TestInitPostgres_Success(t *testing.T) {
	origNew := pgNewWithConfig
	origPing := pgPing
	origPool := Pool
	defer func() {
		pgNewWithConfig = origNew
		pgPing = origPing
		Pool = origPool
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	var capturedPool *pgxpool.Pool
	pgNewWithConfig = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		// We need a real pool object. Create one that won't actually connect.
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			return nil, err
		}
		capturedPool = pool
		return pool, nil
	}

	pgPing = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil // pretend ping succeeded
	}

	err := InitPostgres()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if Pool == nil {
		t.Fatal("Pool should be set after successful init")
	}
	if Pool != capturedPool {
		t.Fatal("Pool should be the pool returned by NewWithConfig")
	}
	// Clean up the pool
	Pool.Close()
}

func TestClosePostgres_NilPool(t *testing.T) {
	origPool := Pool
	defer func() { Pool = origPool }()

	Pool = nil
	ClosePostgres() // should not panic
}

func TestClosePostgres_NonNilPool(t *testing.T) {
	origNew := pgNewWithConfig
	origPing := pgPing
	origPool := Pool
	defer func() {
		pgNewWithConfig = origNew
		pgPing = origPing
		Pool = origPool
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	pgNewWithConfig = func(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
		return pgxpool.NewWithConfig(ctx, cfg)
	}
	pgPing = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	}

	err := InitPostgres()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Pool is non-nil; ClosePostgres should close it without panicking
	ClosePostgres()
}
