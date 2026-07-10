package db

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

//go:embed migrations/*.sql
var embeddedMigrations embed.FS

// migrationFS is the filesystem used to read migration files.
// Tests replace it to simulate read errors.
var migrationFS fs.FS = embeddedMigrations

// Migrator abstracts database execution so migrations can be tested
// without a live PostgreSQL connection.
type Migrator interface {
	Exec(ctx context.Context, sql string, args ...any) error
	QueryRow(ctx context.Context, sql string, args ...any) Scanner
}

// Scanner abstracts pgx Row.Scan for testability.
type Scanner interface {
	Scan(dest ...any) error
}

// pgxMigrator wraps the real pgx pool to implement Migrator.
type pgxMigrator struct{}

func defaultPgxExec(ctx context.Context, sql string, args ...any) error {
	_, err := Pool.Exec(ctx, sql, args...)
	return err
}

func defaultPgxQueryRow(ctx context.Context, sql string, args ...any) Scanner {
	return Pool.QueryRow(ctx, sql, args...)
}

// pgxExec executes SQL via Pool; tests replace it to avoid real connections.
var pgxExec = defaultPgxExec

// pgxQueryRow queries a single row via Pool; tests replace it to avoid real connections.
var pgxQueryRow = defaultPgxQueryRow

func (p *pgxMigrator) Exec(ctx context.Context, sql string, args ...any) error {
	return pgxExec(ctx, sql, args...)
}

func (p *pgxMigrator) QueryRow(ctx context.Context, sql string, args ...any) Scanner {
	return pgxQueryRow(ctx, sql, args...)
}

// migrator is the active Migrator implementation. Tests replace this
// with a mock; production code uses the default pgxMigrator.
var migrator Migrator = &pgxMigrator{}

func RunMigrations() error {
	return RunMigrationsWithMigrator(migrator)
}

// RunMigrationsWithMigrator runs all embedded SQL migrations using the
// provided Migrator. This is the testable core of RunMigrations.
func RunMigrationsWithMigrator(m Migrator) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := m.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	)`)
	if err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	entries, err := fs.ReadDir(migrationFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var names []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		row := m.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)", name)
		var exists bool
		err := row.Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if exists {
			continue
		}

		sql, err := fs.ReadFile(migrationFS, "migrations/"+name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}

		err = m.Exec(ctx, string(sql))
		if err != nil {
			return fmt.Errorf("run migration %s: %w", name, err)
		}

		err = m.Exec(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", name)
		if err != nil {
			return fmt.Errorf("record migration %s: %w", name, err)
		}

		fmt.Printf("Applied migration: %s\n", name)
	}

	return nil
}
