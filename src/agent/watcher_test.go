package main

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatcher_DetectsFileChange(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(filePath, []byte("initial"), 0644)

	var called atomic.Int32
	w := NewWatcher(filePath, 100*time.Millisecond, func() {
		called.Add(1)
	})

	go func() {
		if err := w.Start(); err != nil {
			t.Logf("watcher error: %v", err)
		}
	}()

	// Wait for watcher to initialize
	time.Sleep(200 * time.Millisecond)

	// Modify the file
	os.WriteFile(filePath, []byte("updated"), 0644)

	// Wait for debounce + processing
	time.Sleep(500 * time.Millisecond)

	w.Stop()

	if called.Load() < 1 {
		t.Errorf("onChange called %d times, want >= 1", called.Load())
	}
}

func TestWatcher_Debounce(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(filePath, []byte("initial"), 0644)

	var called atomic.Int32
	w := NewWatcher(filePath, 300*time.Millisecond, func() {
		called.Add(1)
	})

	go func() {
		w.Start()
	}()

	time.Sleep(200 * time.Millisecond)

	// Rapid successive writes — should debounce into one callback
	for i := 0; i < 5; i++ {
		os.WriteFile(filePath, []byte("update-"+string(rune('0'+i))), 0644)
		time.Sleep(50 * time.Millisecond)
	}

	// Wait for debounce to fire
	time.Sleep(600 * time.Millisecond)

	w.Stop()

	// Should be called roughly once (debounce coalesces the 5 writes)
	count := called.Load()
	if count != 1 {
		t.Errorf("onChange called %d times, want 1 (debounced)", count)
	}
}

func TestWatcher_Stop(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(filePath, []byte("initial"), 0644)

	w := NewWatcher(filePath, 100*time.Millisecond, func() {})

	done := make(chan struct{})
	go func() {
		w.Start()
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	w.Stop()

	select {
	case <-done:
		// OK — watcher stopped
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not stop within timeout")
	}
}

func TestNewWatcher(t *testing.T) {
	w := NewWatcher("/tmp/test.md", 500*time.Millisecond, func() {})
	if w.filePath != "/tmp/test.md" {
		t.Errorf("filePath = %q, want %q", w.filePath, "/tmp/test.md")
	}
	if w.debounce != 500*time.Millisecond {
		t.Errorf("debounce = %v, want 500ms", w.debounce)
	}
}
