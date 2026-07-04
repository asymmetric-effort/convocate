//go:build integration

package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultTestDSN = "postgres://postgres@postgresql.data-layer.svc:5432/convocate_test?sslmode=disable"

func testDSN() string {
	if v := os.Getenv("TEST_DATABASE_URL"); v != "" {
		return v
	}
	return defaultTestDSN
}

// ensureTestDatabase connects to the default "postgres" database and creates
// convocate_test if it does not exist.
func ensureTestDatabase(t *testing.T) {
	t.Helper()

	dsn := testDSN()
	// Derive admin DSN by replacing the database name with "postgres"
	adminDSN := replaceDBName(dsn, "postgres")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, adminDSN)
	if err != nil {
		t.Skipf("cannot connect to postgres admin db (skipping): %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Skipf("cannot reach PostgreSQL (skipping): %v", err)
	}

	var exists bool
	err = pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = 'convocate_test')").Scan(&exists)
	if err != nil {
		t.Fatalf("query pg_database: %v", err)
	}
	if !exists {
		_, err = pool.Exec(ctx, "CREATE DATABASE convocate_test")
		if err != nil {
			t.Fatalf("create convocate_test database: %v", err)
		}
		t.Log("created convocate_test database")
	}
}

// replaceDBName swaps the database name in a postgres DSN.
// Handles both "postgres://user@host:port/dbname?..." and bare forms.
func replaceDBName(dsn, newDB string) string {
	// Find the last '/' before '?' (or end) and replace the db name
	qIdx := len(dsn)
	for i, c := range dsn {
		if c == '?' {
			qIdx = i
			break
		}
	}
	// Find the last '/' before query string
	slashIdx := -1
	for i := qIdx - 1; i >= 0; i-- {
		if dsn[i] == '/' {
			slashIdx = i
			break
		}
	}
	if slashIdx < 0 {
		return dsn
	}
	return dsn[:slashIdx+1] + newDB + dsn[qIdx:]
}

func setupIntegrationPG(t *testing.T) {
	t.Helper()
	ensureTestDatabase(t)

	os.Setenv("DATABASE_URL", testDSN())
	t.Cleanup(func() {
		os.Unsetenv("DATABASE_URL")
	})
}

func TestIntegration_InitPostgres(t *testing.T) {
	setupIntegrationPG(t)

	err := InitPostgres()
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL (skipping): %v", err)
	}
	t.Cleanup(func() {
		ClosePostgres()
		Pool = nil
	})

	if Pool == nil {
		t.Fatal("Pool should not be nil after successful init")
	}

	// Verify we can ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Pool.Ping(ctx); err != nil {
		t.Fatalf("ping after init: %v", err)
	}
	t.Log("InitPostgres connected successfully")
}

func TestIntegration_RunMigrations(t *testing.T) {
	setupIntegrationPG(t)

	err := InitPostgres()
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL (skipping): %v", err)
	}
	t.Cleanup(func() {
		ClosePostgres()
		Pool = nil
	})

	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations failed: %v", err)
	}

	// Verify expected tables exist
	expectedTables := []string{
		"schema_migrations",
		"users",
		"groups",
		"roles",
		"user_groups",
		"group_roles",
		"global_settings",
		"node_notes",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, table := range expectedTables {
		var exists bool
		err := Pool.QueryRow(ctx,
			"SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_schema = 'public' AND table_name = $1)",
			table,
		).Scan(&exists)
		if err != nil {
			t.Fatalf("check table %s: %v", table, err)
		}
		if !exists {
			t.Errorf("expected table %q to exist after migrations", table)
		} else {
			t.Logf("table %q exists", table)
		}
	}

	// Verify migrations are idempotent — run again
	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations (second run) failed: %v", err)
	}
	t.Log("migrations are idempotent")
}

func TestIntegration_CRUD_NodeNotes(t *testing.T) {
	setupIntegrationPG(t)

	err := InitPostgres()
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL (skipping): %v", err)
	}
	t.Cleanup(func() {
		ClosePostgres()
		Pool = nil
	})

	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	testNodeID := fmt.Sprintf("test-node-%d", time.Now().UnixNano())
	testAuthor := "integration-test"
	testText := "This is a test note"

	// INSERT
	var noteID int
	err = Pool.QueryRow(ctx,
		"INSERT INTO node_notes (node_id, author, text) VALUES ($1, $2, $3) RETURNING id",
		testNodeID, testAuthor, testText,
	).Scan(&noteID)
	if err != nil {
		t.Fatalf("INSERT node_notes: %v", err)
	}
	t.Logf("inserted note id=%d", noteID)

	// SELECT
	var gotAuthor, gotText string
	err = Pool.QueryRow(ctx,
		"SELECT author, text FROM node_notes WHERE id = $1",
		noteID,
	).Scan(&gotAuthor, &gotText)
	if err != nil {
		t.Fatalf("SELECT node_notes: %v", err)
	}
	if gotAuthor != testAuthor {
		t.Errorf("author = %q, want %q", gotAuthor, testAuthor)
	}
	if gotText != testText {
		t.Errorf("text = %q, want %q", gotText, testText)
	}
	t.Log("SELECT returned correct data")

	// UPDATE
	updatedText := "Updated test note"
	_, err = Pool.Exec(ctx,
		"UPDATE node_notes SET text = $1 WHERE id = $2",
		updatedText, noteID,
	)
	if err != nil {
		t.Fatalf("UPDATE node_notes: %v", err)
	}

	err = Pool.QueryRow(ctx,
		"SELECT text FROM node_notes WHERE id = $1",
		noteID,
	).Scan(&gotText)
	if err != nil {
		t.Fatalf("SELECT after UPDATE: %v", err)
	}
	if gotText != updatedText {
		t.Errorf("text after update = %q, want %q", gotText, updatedText)
	}
	t.Log("UPDATE verified")

	// DELETE
	_, err = Pool.Exec(ctx, "DELETE FROM node_notes WHERE id = $1", noteID)
	if err != nil {
		t.Fatalf("DELETE node_notes: %v", err)
	}

	err = Pool.QueryRow(ctx,
		"SELECT text FROM node_notes WHERE id = $1",
		noteID,
	).Scan(&gotText)
	if err == nil {
		t.Fatal("expected error after DELETE, got nil")
	}
	t.Log("DELETE verified — row not found after deletion")
}

func TestIntegration_ClosePostgres(t *testing.T) {
	setupIntegrationPG(t)

	err := InitPostgres()
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL (skipping): %v", err)
	}

	// Pool should be valid before close
	if Pool == nil {
		t.Fatal("Pool should not be nil")
	}

	ClosePostgres()
	// After close, pool stats should show it's closed
	// pgxpool doesn't expose an IsClosed method, but Ping should fail
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = Pool.Ping(ctx)
	if err == nil {
		t.Error("expected Ping to fail after ClosePostgres")
	} else {
		t.Logf("Ping after close returned expected error: %v", err)
	}
	Pool = nil
}

func TestIntegration_WrongPort(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://postgres@postgresql.data-layer.svc:59999/convocate_test?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	if err == nil {
		ClosePostgres()
		Pool = nil
		t.Fatal("expected error for wrong port")
	}
	t.Logf("wrong port error: %v", err)
}

func TestIntegration_WrongDatabase(t *testing.T) {
	os.Setenv("DATABASE_URL", "postgres://postgres@postgresql.data-layer.svc:5432/nonexistent_db_12345?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	if err == nil {
		ClosePostgres()
		Pool = nil
		t.Fatal("expected error for nonexistent database")
	}
	t.Logf("wrong database error: %v", err)
}

func TestIntegration_MigrationsRecordedInSchemaTable(t *testing.T) {
	setupIntegrationPG(t)

	err := InitPostgres()
	if err != nil {
		t.Skipf("cannot connect to PostgreSQL (skipping): %v", err)
	}
	t.Cleanup(func() {
		ClosePostgres()
		Pool = nil
	})

	err = RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := Pool.Query(ctx, "SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer rows.Close()

	var versions []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows iteration: %v", err)
	}

	if len(versions) == 0 {
		t.Fatal("expected at least one migration recorded")
	}
	for _, v := range versions {
		t.Logf("recorded migration: %s", v)
	}
}
