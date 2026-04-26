package main

import (
	"log"
	"os"
)

// newStatusLogger returns a logger that writes to stderr — the shell-side
// status listener runs as a daemon, so stderr ends up in the systemd
// journal. Kept as a small helper so tests can replace it easily.
var newStatusLogger = func() *log.Logger {
	return log.New(os.Stderr, "", log.LstdFlags|log.Lmicroseconds)
}
