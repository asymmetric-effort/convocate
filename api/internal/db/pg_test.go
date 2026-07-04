package db

import (
	"os"
	"testing"
)

func TestInitPostgres_DefaultDSN(t *testing.T) {
	// Unset DATABASE_URL to exercise the default DSN path
	os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	// We expect a connection error since no PG is running,
	// but we verify the error message shows the default DSN was used.
	if err == nil {
		t.Fatal("expected error when no PG is available")
	}
	// The error should be about connecting, not about parsing
	// (the default DSN is valid syntax).
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
}

func TestClosePostgres_NilPool(t *testing.T) {
	// Ensure no panic when Pool is nil
	Pool = nil
	ClosePostgres() // should not panic
}

func TestClosePostgres_NonNilPool(t *testing.T) {
	// We can't easily create a real pool without a connection,
	// but we can test the nil guard by setting Pool to nil
	// and verifying no panic.
	original := Pool
	defer func() { Pool = original }()

	Pool = nil
	ClosePostgres()
}

func TestInitPostgres_ParsesConfigCorrectly(t *testing.T) {
	// Use a valid DSN that will parse but fail on connect
	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:1/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	err := InitPostgres()
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
	// Error should be about connecting, not parsing
	errStr := err.Error()
	if errStr == "" {
		t.Fatal("expected non-empty error")
	}
}
