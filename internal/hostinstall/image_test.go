package hostinstall

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTransferImage_RequiresTag(t *testing.T) {
	m := &mockRunner{}
	err := TransferImage(context.Background(), m, "", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "tag is required") {
		t.Errorf("expected tag-required error, got %v", err)
	}
	if len(m.cmds) != 0 || len(m.copies) != 0 {
		t.Error("no work should happen with empty tag")
	}
}

// TestTransferImage_DockerSaveMissing exercises the error path when
// `docker save` isn't available (or the image doesn't exist). Under any
// environment we can reach — including CI without docker — exec.Command
// against a nonexistent binary or a missing image fails.
func TestTransferImage_DockerSaveMissing(t *testing.T) {
	// Fake a PATH with no docker; saveAndGzip will error on exec.
	t.Setenv("PATH", "/nonexistent")
	m := &mockRunner{}
	err := TransferImage(context.Background(), m, "convocate:v0.0.0", io.Discard)
	if err == nil {
		t.Error("expected error when docker is unavailable")
	}
	if !strings.Contains(err.Error(), "save+gzip") && !strings.Contains(err.Error(), "docker save") {
		t.Errorf("error = %q, want a docker-save failure hint", err)
	}
	if len(m.copies) != 0 {
		t.Error("no copy should run when save fails")
	}
}

// TestSaveAndGzip_HashMatchesSum feeds the saveAndGzip path a fake
// "docker save" by preparing a known input and reproducing the hash
// manually. We can't drive the real saveAndGzip in a no-docker env, but
// we can validate the inner invariant: the SHA-256 we emit equals
// sha256(gzip(input)) which is what the remote `sha256sum` computes.
func TestSaveAndGzip_HashMatchesSum(t *testing.T) {
	// Pipe the same bytes through gzip manually and compare hashes.
	want := []byte("hello world — a tiny pretend tarball body\n")

	// Reference: gzip bytes with the same options and hash them.
	var ref bytes.Buffer
	gzRef := gzip.NewWriter(&ref)
	if _, err := gzRef.Write(want); err != nil {
		t.Fatal(err)
	}
	_ = gzRef.Close()
	refHash := sha256.Sum256(ref.Bytes())

	// Mirror saveAndGzip's write path without invoking docker: write the
	// payload through a gzip.Writer wrapping MultiWriter(file, hasher).
	dir := t.TempDir()
	out, err := os.Create(filepath.Join(dir, "x.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	hasher := sha256.New()
	counter := &countingWriter{w: io.MultiWriter(out, hasher)}
	gz := gzip.NewWriter(counter)
	if _, err := gz.Write(want); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(hasher.Sum(nil)) != hex.EncodeToString(refHash[:]) {
		t.Errorf("gzip hash mismatch between saveAndGzip-style and reference")
	}
}

func TestCountingWriter_TalliesBytes(t *testing.T) {
	c := &countingWriter{w: io.Discard}
	n, err := c.Write([]byte("abc"))
	if err != nil || n != 3 {
		t.Errorf("Write: n=%d err=%v", n, err)
	}
	if _, err := c.Write([]byte("defgh")); err != nil {
		t.Fatal(err)
	}
	if c.n != 8 {
		t.Errorf("total = %d, want 8", c.n)
	}
}

// TestTransferImage_HappyPath needs a real docker or a stub binary on
// PATH that behaves like `docker save <tag>`. We stand one up by
// writing a tiny shell script to a tempdir and prepending that tempdir
// to PATH. The script emits a fixed byte sequence so the remote
// sha256sum command path can be exercised end-to-end against the
// mockRunner's captured command text.
func TestTransferImage_HappyPath(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("no /bin/sh available")
	}
	binDir := t.TempDir()
	stub := filepath.Join(binDir, "docker")
	// Stub prints a tiny fixed payload to stdout when asked to save,
	// matching docker's "give me bytes on stdout" contract. Anything
	// else exits 1 so surprising invocations fail visibly.
	if err := os.WriteFile(stub, []byte("#!/bin/sh\ncase \"$1\" in\n  save) printf 'fake-tarball-content\\n' ;;\n  *) exit 1 ;;\nesac\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", binDir)

	m := &mockRunner{}
	err := TransferImage(context.Background(), m, "convocate:v1.2.3", io.Discard)
	if err != nil {
		t.Fatalf("TransferImage: %v", err)
	}
	// Expect exactly one copy to /tmp/convocate-image-<16 hex>.tar.gz
	if len(m.copies) != 1 {
		t.Fatalf("copies = %d, want 1", len(m.copies))
	}
	if !strings.HasPrefix(m.copies[0].Dst, "/tmp/convocate-image-") {
		t.Errorf("dst = %q", m.copies[0].Dst)
	}
	if m.copies[0].Mode != 0600 {
		t.Errorf("mode = %o, want 0600", m.copies[0].Mode)
	}
	// Expect one Run call containing sha256 check + docker load.
	if len(m.cmds) != 1 {
		t.Fatalf("cmds = %d, want 1", len(m.cmds))
	}
	body := m.cmds[0].Cmd
	for _, want := range []string{"sha256sum", "docker load", "gunzip -c"} {
		if !strings.Contains(body, want) {
			t.Errorf("remote script missing %q: %s", want, body)
		}
	}
	if !m.cmds[0].Opts.Sudo {
		t.Error("remote load should run under sudo")
	}
}
