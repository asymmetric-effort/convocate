package assets

import (
	"strings"
	"testing"
)

func TestDockerfile(t *testing.T) {
	data, err := Dockerfile()
	if err != nil {
		t.Fatalf("Dockerfile() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Dockerfile() returned empty data")
	}
	if !strings.Contains(string(data), "FROM") {
		t.Error("Dockerfile does not contain FROM directive")
	}
}

func TestEntrypoint(t *testing.T) {
	data, err := Entrypoint()
	if err != nil {
		t.Fatalf("Entrypoint() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Entrypoint() returned empty data")
	}
	if !strings.HasPrefix(string(data), "#!/bin/bash") {
		t.Error("entrypoint.sh does not start with shebang")
	}
}

func TestClaudeMD(t *testing.T) {
	data, err := ClaudeMD()
	if err != nil {
		t.Fatalf("ClaudeMD() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("ClaudeMD() returned empty data")
	}
	if !strings.Contains(string(data), "Claude Session") {
		t.Error("CLAUDE.md does not contain expected content")
	}
}
