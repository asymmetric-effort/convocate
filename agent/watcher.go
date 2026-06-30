// Configuration file watcher for CLAUDE.md changes.
// Uses fsnotify to detect ConfigMap updates and restarts Claude CLI.

package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors a file for changes and calls a callback on modification.
// Uses debouncing to coalesce rapid ConfigMap update events.
type Watcher struct {
	filePath string
	debounce time.Duration
	onChange func()
	stop     chan struct{}
}

// NewWatcher creates a file watcher for the given path.
// onChange is called (debounced) when the file changes.
func NewWatcher(filePath string, debounce time.Duration, onChange func()) *Watcher {
	return &Watcher{
		filePath: filePath,
		debounce: debounce,
		onChange: onChange,
		stop:     make(chan struct{}),
	}
}

// Start begins watching the file. Blocks until Stop is called or an error occurs.
func (w *Watcher) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Watch the parent directory — K8s ConfigMap updates replace the
	// symlink target, which generates Create events on the directory,
	// not Write events on the file.
	dir := filepath.Dir(w.filePath)
	if err := watcher.Add(dir); err != nil {
		return err
	}

	baseName := filepath.Base(w.filePath)
	var timer *time.Timer

	log.Printf("[watcher] Watching %s for changes (debounce=%v)", w.filePath, w.debounce)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			// Filter for events on the target file
			if filepath.Base(event.Name) != baseName {
				continue
			}
			if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
				continue
			}

			log.Printf("[watcher] Detected change on %s (op=%v)", event.Name, event.Op)

			// Debounce: reset the timer on each event
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, func() {
				log.Printf("[watcher] Debounce elapsed, triggering restart")
				w.onChange()
			})

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Printf("[watcher] Error: %v", err)

		case <-w.stop:
			if timer != nil {
				timer.Stop()
			}
			return nil
		}
	}
}

// Stop signals the watcher to exit.
func (w *Watcher) Stop() {
	close(w.stop)
}
