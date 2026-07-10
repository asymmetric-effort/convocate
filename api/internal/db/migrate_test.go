package db

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// mockScanner implements Scanner for tests.
type mockScanner struct {
	exists bool
	err    error
}

func (s *mockScanner) Scan(dest ...any) error {
	if s.err != nil {
		return s.err
	}
	if len(dest) > 0 {
		if b, ok := dest[0].(*bool); ok {
			*b = s.exists
		}
	}
	return nil
}

// mockMigrator records calls and can be configured to return errors.
type mockMigrator struct {
	mu         sync.Mutex
	execCalls  []execCall
	queryCalls []string

	// Controls
	execErr      error            // global exec error
	execErrOnSQL map[string]error // per-SQL exec error (substring match)
	queryExists  map[string]bool  // version -> exists
	queryErr     error            // global query error
}

type execCall struct {
	SQL  string
	Args []any
}

func newMockMigrator() *mockMigrator {
	return &mockMigrator{
		execErrOnSQL: make(map[string]error),
		queryExists:  make(map[string]bool),
	}
}

func (m *mockMigrator) Exec(ctx context.Context, sql string, args ...any) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.execCalls = append(m.execCalls, execCall{SQL: sql, Args: args})
	if m.execErr != nil {
		return m.execErr
	}
	for substr, err := range m.execErrOnSQL {
		if strings.Contains(sql, substr) {
			return err
		}
	}
	return nil
}

func (m *mockMigrator) QueryRow(ctx context.Context, sql string, args ...any) Scanner {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.queryCalls = append(m.queryCalls, sql)
	if m.queryErr != nil {
		return &mockScanner{err: m.queryErr}
	}
	// Check if any arg matches a version in queryExists
	for _, arg := range args {
		if v, ok := arg.(string); ok {
			if exists, found := m.queryExists[v]; found {
				return &mockScanner{exists: exists}
			}
		}
	}
	return &mockScanner{exists: false}
}

func (m *mockMigrator) getExecCalls() []execCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]execCall, len(m.execCalls))
	copy(cp, m.execCalls)
	return cp
}

func TestRunMigrationsWithMigrator_Success(t *testing.T) {
	m := newMockMigrator()

	err := RunMigrationsWithMigrator(m)
	if err != nil {
		t.Fatalf("RunMigrationsWithMigrator: %v", err)
	}

	calls := m.getExecCalls()
	if len(calls) == 0 {
		t.Fatal("expected at least one exec call")
	}

	// First call should be CREATE TABLE for schema_migrations
	if !strings.Contains(calls[0].SQL, "schema_migrations") {
		t.Fatalf("first exec should create schema_migrations, got: %s", calls[0].SQL)
	}

	// Should have exec calls for each migration SQL + INSERT for each
	// We have 5 migration files, so expect: 1 (create table) + 5 (migration SQL) + 5 (INSERT) = 11
	expectedCalls := 1 + 5 + 5
	if len(calls) != expectedCalls {
		t.Fatalf("expected %d exec calls, got %d", expectedCalls, len(calls))
	}

	// Verify migrations run in sorted order
	migrationInserts := []string{}
	for _, c := range calls[1:] {
		if strings.Contains(c.SQL, "INSERT INTO schema_migrations") {
			if len(c.Args) > 0 {
				migrationInserts = append(migrationInserts, fmt.Sprintf("%v", c.Args[0]))
			}
		}
	}
	if len(migrationInserts) != 5 {
		t.Fatalf("expected 5 migration inserts, got %d", len(migrationInserts))
	}
	// Verify order
	for i := 1; i < len(migrationInserts); i++ {
		if migrationInserts[i] < migrationInserts[i-1] {
			t.Fatalf("migrations not in sorted order: %v", migrationInserts)
		}
	}
}

func TestRunMigrationsWithMigrator_CreateTableError(t *testing.T) {
	m := newMockMigrator()
	m.execErr = fmt.Errorf("connection refused")

	err := RunMigrationsWithMigrator(m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create migrations table") {
		t.Fatalf("expected 'create migrations table' error, got: %v", err)
	}
}

func TestRunMigrationsWithMigrator_Idempotent(t *testing.T) {
	m := newMockMigrator()
	// Mark all migrations as already applied
	m.queryExists["001_users_and_auth.sql"] = true
	m.queryExists["002_nodes_and_notes.sql"] = true
	m.queryExists["003_tickets.sql"] = true
	m.queryExists["004_repos_and_projects.sql"] = true
	m.queryExists["005_boards.sql"] = true

	err := RunMigrationsWithMigrator(m)
	if err != nil {
		t.Fatalf("RunMigrationsWithMigrator: %v", err)
	}

	calls := m.getExecCalls()
	// Should only have the CREATE TABLE call, no migration SQL or INSERTs
	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call (create table only), got %d", len(calls))
	}
}

func TestRunMigrationsWithMigrator_PartiallyApplied(t *testing.T) {
	m := newMockMigrator()
	// First 2 already applied
	m.queryExists["001_users_and_auth.sql"] = true
	m.queryExists["002_nodes_and_notes.sql"] = true

	err := RunMigrationsWithMigrator(m)
	if err != nil {
		t.Fatalf("RunMigrationsWithMigrator: %v", err)
	}

	calls := m.getExecCalls()
	// 1 (create table) + 3 (remaining migration SQL) + 3 (INSERT) = 7
	if len(calls) != 7 {
		t.Fatalf("expected 7 exec calls, got %d", len(calls))
	}
}

func TestRunMigrationsWithMigrator_QueryRowError(t *testing.T) {
	m := newMockMigrator()
	m.queryErr = fmt.Errorf("query failed")

	err := RunMigrationsWithMigrator(m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "check migration") {
		t.Fatalf("expected 'check migration' error, got: %v", err)
	}
}

func TestRunMigrationsWithMigrator_MigrationExecError(t *testing.T) {
	m := newMockMigrator()
	// Let the first exec (CREATE TABLE) succeed, but fail on actual SQL content.
	// We need a more targeted approach: fail on the migration SQL content.
	// Migration files contain CREATE TABLE statements for app tables.
	// Use a flag after the first call.

	callCount := 0
	origExec := m.Exec
	_ = origExec
	m2 := &countingMigrator{
		inner:      m,
		failOnCall: 2, // fail on second exec (first migration SQL)
		failErr:    fmt.Errorf("syntax error in migration"),
	}

	err := RunMigrationsWithMigrator(m2)
	if err == nil {
		t.Fatal("expected error")
	}
	_ = callCount
	if !strings.Contains(err.Error(), "run migration") {
		t.Fatalf("expected 'run migration' error, got: %v", err)
	}
}

func TestRunMigrationsWithMigrator_RecordMigrationError(t *testing.T) {
	m := &countingMigrator{
		inner:      newMockMigrator(),
		failOnCall: 3, // fail on third exec (INSERT INTO schema_migrations)
		failErr:    fmt.Errorf("insert failed"),
	}

	err := RunMigrationsWithMigrator(m)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "record migration") {
		t.Fatalf("expected 'record migration' error, got: %v", err)
	}
}

// countingMigrator wraps a mock and fails on a specific exec call number.
type countingMigrator struct {
	inner      *mockMigrator
	mu         sync.Mutex
	count      int
	failOnCall int
	failErr    error
}

func (c *countingMigrator) Exec(ctx context.Context, sql string, args ...any) error {
	c.mu.Lock()
	c.count++
	n := c.count
	c.mu.Unlock()
	if n == c.failOnCall {
		return c.failErr
	}
	return c.inner.Exec(ctx, sql, args...)
}

func (c *countingMigrator) QueryRow(ctx context.Context, sql string, args ...any) Scanner {
	return c.inner.QueryRow(ctx, sql, args...)
}

func TestRunMigrations_UsesDefaultMigrator(t *testing.T) {
	// Save and restore
	orig := migrator
	defer func() { migrator = orig }()

	m := newMockMigrator()
	migrator = m

	err := RunMigrations()
	if err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	calls := m.getExecCalls()
	if len(calls) == 0 {
		t.Fatal("expected exec calls through default migrator")
	}
}

func TestDefaultPgxExec_WithPool(t *testing.T) {
	origPool := Pool
	origNew := pgNewWithConfig
	origPing := pgPing
	defer func() {
		if Pool != nil {
			Pool.Close()
		}
		Pool = origPool
		pgNewWithConfig = origNew
		pgPing = origPing
	}()

	// Create a real pool that won't be connected
	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	pgPing = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	}

	err := InitPostgres()
	if err != nil {
		t.Fatalf("expected no error with mocked ping, got: %v", err)
	}

	// Now call defaultPgxExec — it will fail because the pool isn't really connected,
	// but it will cover the code path.
	ctx := context.Background()
	err = defaultPgxExec(ctx, "SELECT 1")
	// We expect an error (pool not connected to real PG)
	if err == nil {
		t.Fatal("expected error from defaultPgxExec on non-connected pool")
	}
}

func TestDefaultPgxQueryRow_WithPool(t *testing.T) {
	origPool := Pool
	origNew := pgNewWithConfig
	origPing := pgPing
	defer func() {
		if Pool != nil {
			Pool.Close()
		}
		Pool = origPool
		pgNewWithConfig = origNew
		pgPing = origPing
	}()

	os.Setenv("DATABASE_URL", "postgres://user:pass@127.0.0.1:5432/testdb?sslmode=disable")
	defer os.Unsetenv("DATABASE_URL")

	pgPing = func(ctx context.Context, pool *pgxpool.Pool) error {
		return nil
	}

	err := InitPostgres()
	if err != nil {
		t.Fatalf("expected no error with mocked ping, got: %v", err)
	}

	ctx := context.Background()
	row := defaultPgxQueryRow(ctx, "SELECT 1")
	// row.Scan will fail because the pool isn't really connected
	var result int
	err = row.Scan(&result)
	if err == nil {
		t.Fatal("expected error from defaultPgxQueryRow on non-connected pool")
	}
}

func TestPgxMigrator_Exec(t *testing.T) {
	origExec := pgxExec
	defer func() { pgxExec = origExec }()

	var capturedSQL string
	var capturedArgs []any
	pgxExec = func(ctx context.Context, sql string, args ...any) error {
		capturedSQL = sql
		capturedArgs = args
		return nil
	}

	m := &pgxMigrator{}
	ctx := context.Background()
	err := m.Exec(ctx, "SELECT 1", "arg1", "arg2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedSQL != "SELECT 1" {
		t.Fatalf("expected SQL 'SELECT 1', got %q", capturedSQL)
	}
	if len(capturedArgs) != 2 {
		t.Fatalf("expected 2 args, got %d", len(capturedArgs))
	}
}

func TestPgxMigrator_Exec_Error(t *testing.T) {
	origExec := pgxExec
	defer func() { pgxExec = origExec }()

	pgxExec = func(ctx context.Context, sql string, args ...any) error {
		return fmt.Errorf("exec failed")
	}

	m := &pgxMigrator{}
	ctx := context.Background()
	err := m.Exec(ctx, "SELECT 1")
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "exec failed" {
		t.Fatalf("expected 'exec failed', got: %v", err)
	}
}

func TestPgxMigrator_QueryRow(t *testing.T) {
	origQueryRow := pgxQueryRow
	defer func() { pgxQueryRow = origQueryRow }()

	pgxQueryRow = func(ctx context.Context, sql string, args ...any) Scanner {
		return &mockScanner{exists: true}
	}

	m := &pgxMigrator{}
	ctx := context.Background()
	row := m.QueryRow(ctx, "SELECT EXISTS(...)", "version1")
	var exists bool
	err := row.Scan(&exists)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
}

func TestPgxMigrator_QueryRow_Error(t *testing.T) {
	origQueryRow := pgxQueryRow
	defer func() { pgxQueryRow = origQueryRow }()

	pgxQueryRow = func(ctx context.Context, sql string, args ...any) Scanner {
		return &mockScanner{err: fmt.Errorf("query failed")}
	}

	m := &pgxMigrator{}
	ctx := context.Background()
	row := m.QueryRow(ctx, "SELECT EXISTS(...)")
	var exists bool
	err := row.Scan(&exists)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "query failed" {
		t.Fatalf("expected 'query failed', got: %v", err)
	}
}

// brokenReadFileFS wraps an fs.FS but returns an error for ReadFile on any
// file under "migrations/".
type brokenReadFileFS struct {
	inner fs.FS
}

func (b *brokenReadFileFS) Open(name string) (fs.File, error) {
	return b.inner.Open(name)
}

// ReadDir implements fs.ReadDirFS so fs.ReadDir works.
func (b *brokenReadFileFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return fs.ReadDir(b.inner, name)
}

// ReadFile implements fs.ReadFileFS but always fails for migration files.
func (b *brokenReadFileFS) ReadFile(name string) ([]byte, error) {
	if strings.HasPrefix(name, "migrations/") {
		return nil, fmt.Errorf("disk I/O error")
	}
	return fs.ReadFile(b.inner, name)
}

func TestRunMigrationsWithMigrator_ReadFileError(t *testing.T) {
	origFS := migrationFS
	defer func() { migrationFS = origFS }()

	migrationFS = &brokenReadFileFS{inner: embeddedMigrations}

	m := newMockMigrator()
	err := RunMigrationsWithMigrator(m)
	if err == nil {
		t.Fatal("expected error from ReadFile failure")
	}
	if !strings.Contains(err.Error(), "read migration") {
		t.Fatalf("expected 'read migration' error, got: %v", err)
	}
}

// brokenReadDirFS always fails on ReadDir.
type brokenReadDirFS struct{}

func (b *brokenReadDirFS) Open(name string) (fs.File, error) {
	return nil, fmt.Errorf("open failed")
}

func TestRunMigrationsWithMigrator_ReadDirError(t *testing.T) {
	origFS := migrationFS
	defer func() { migrationFS = origFS }()

	migrationFS = &brokenReadDirFS{}

	m := newMockMigrator()
	err := RunMigrationsWithMigrator(m)
	if err == nil {
		t.Fatal("expected error from ReadDir failure")
	}
	if !strings.Contains(err.Error(), "read migrations dir") {
		t.Fatalf("expected 'read migrations dir' error, got: %v", err)
	}
}

// emptyDirFS has a migrations directory but no .sql files.
type emptyDirFS struct{}

type fakeDirEntry struct {
	name string
}

func (f *fakeDirEntry) Name() string               { return f.name }
func (f *fakeDirEntry) IsDir() bool                { return false }
func (f *fakeDirEntry) Type() fs.FileMode          { return 0 }
func (f *fakeDirEntry) Info() (fs.FileInfo, error) { return nil, nil }

type emptyDirFile struct{}

func (e *emptyDirFile) Read([]byte) (int, error)   { return 0, fmt.Errorf("not a file") }
func (e *emptyDirFile) Close() error               { return nil }
func (e *emptyDirFile) Stat() (fs.FileInfo, error) { return &emptyDirInfo{}, nil }
func (e *emptyDirFile) ReadDir(n int) ([]fs.DirEntry, error) {
	// Return entries with non-.sql names
	return []fs.DirEntry{&fakeDirEntry{name: "README.txt"}}, nil
}

type emptyDirInfo struct{}

func (e *emptyDirInfo) Name() string       { return "migrations" }
func (e *emptyDirInfo) Size() int64        { return 0 }
func (e *emptyDirInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (e *emptyDirInfo) ModTime() time.Time { return time.Time{} }
func (e *emptyDirInfo) IsDir() bool        { return true }
func (e *emptyDirInfo) Sys() any           { return nil }

func (ed *emptyDirFS) Open(name string) (fs.File, error) {
	if name == "migrations" {
		return &emptyDirFile{}, nil
	}
	return nil, fmt.Errorf("not found")
}

func TestRunMigrationsWithMigrator_NoSQLFiles(t *testing.T) {
	origFS := migrationFS
	defer func() { migrationFS = origFS }()

	migrationFS = &emptyDirFS{}

	m := newMockMigrator()
	err := RunMigrationsWithMigrator(m)
	if err != nil {
		t.Fatalf("expected no error when no .sql files found, got: %v", err)
	}

	calls := m.getExecCalls()
	// Should only have the CREATE TABLE call
	if len(calls) != 1 {
		t.Fatalf("expected 1 exec call (create table only), got %d", len(calls))
	}
}
