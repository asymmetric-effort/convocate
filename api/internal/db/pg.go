package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func defaultPgNewWithConfig(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	return pgxpool.NewWithConfig(ctx, cfg)
}

func defaultPgPing(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// pgNewWithConfig is the pool constructor; tests replace it to avoid real connections.
var pgNewWithConfig = defaultPgNewWithConfig

// pgPing pings the pool; tests replace it to simulate ping failures.
var pgPing = defaultPgPing

func InitPostgres() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres@postgresql.data-layer.svc:5432/convocate?sslmode=disable"
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse database url: %w", err)
	}

	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = 30 * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgNewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pgPing(ctx, pool); err != nil {
		return fmt.Errorf("ping postgres: %w", err)
	}

	Pool = pool
	return nil
}

func ClosePostgres() {
	if Pool != nil {
		Pool.Close()
	}
}
