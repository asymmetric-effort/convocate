// Package diskspace provides disk space availability checks for file creation.
package diskspace

import (
	"fmt"
	"path/filepath"
	"syscall"
)

const reserveBytes = 10 * 1024 * 1024 // 10 MB

// CheckForFile verifies that the filesystem containing dir has at least
// fileSize + 10 MB of free space. If dir does not exist, it walks up to
// the nearest existing ancestor to perform the check.
func CheckForFile(dir string, fileSize int64) error {
	checkDir, err := nearestExistingDir(dir)
	if err != nil {
		return fmt.Errorf("cannot determine filesystem for %q: %w", dir, err)
	}

	var stat syscall.Statfs_t
	if err := syscall.Statfs(checkDir, &stat); err != nil {
		return fmt.Errorf("cannot check disk space on %q: %w", checkDir, err)
	}

	available := int64(stat.Bavail) * int64(stat.Bsize)
	required := fileSize + reserveBytes

	if available < required {
		availMB := float64(available) / (1024 * 1024)
		reqMB := float64(required) / (1024 * 1024)
		return fmt.Errorf(
			"not enough disk space in %q: %.1f MB available but %.1f MB required "+
				"(file size + 10 MB reserve). Please free up some space and try again",
			dir, availMB, reqMB,
		)
	}

	return nil
}

// nearestExistingDir walks up from dir until it finds a directory that exists.
func nearestExistingDir(dir string) (string, error) {
	dir = filepath.Clean(dir)
	for {
		err := syscall.Stat(dir, &syscall.Stat_t{})
		if err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no existing ancestor found for %q", dir)
		}
		dir = parent
	}
}
