package hostinstall

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// TransferImage streams the local Docker image `tag` to the remote host
// represented by r. Flow:
//
//  1. docker save <tag> | gzip -n → local temp file, computing SHA-256
//     while writing.
//  2. CopyFile the gz blob to /tmp on the agent.
//  3. Run `sha256sum` on the agent and compare to the local digest. On
//     mismatch, remove the file and error out — we won't load a
//     corrupted tarball into docker.
//  4. `gunzip -c | docker load` to register the image on the agent.
//  5. rm the temp file on both ends.
//
// The shell host must already have the image present (claude-shell
// install built it). A missing image surfaces as "docker save" failing
// with a clear error.
func TransferImage(ctx context.Context, r Runner, tag string, log io.Writer) error {
	if tag == "" {
		return fmt.Errorf("transfer image: tag is required")
	}
	if log == nil {
		log = io.Discard
	}

	localDir, err := os.MkdirTemp("", "claude-image-*")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(localDir)
	localPath := filepath.Join(localDir, "image.tar.gz")

	fmt.Fprintf(log, "[claude-host] saving %s locally + gzipping...\n", tag)
	digest, size, err := saveAndGzip(ctx, tag, localPath)
	if err != nil {
		return fmt.Errorf("save+gzip %s: %w", tag, err)
	}
	hexDigest := hex.EncodeToString(digest)
	fmt.Fprintf(log, "[claude-host] local tarball: %d bytes, sha256=%s\n", size, hexDigest)

	// Remote staging path — keyed by digest prefix so repeated transfers
	// don't stomp on each other if one is interrupted mid-flight.
	remotePath := fmt.Sprintf("/tmp/claude-image-%s.tar.gz", hexDigest[:16])
	fmt.Fprintf(log, "[claude-host] uploading to %s...\n", remotePath)
	if err := r.CopyFile(ctx, localPath, remotePath, 0600); err != nil {
		return fmt.Errorf("upload to %s: %w", remotePath, err)
	}

	// Verify + load. On mismatch we remove the remote file so a subsequent
	// retry starts clean.
	script := fmt.Sprintf(`set -e
remote_sha=$(sha256sum %[1]q | awk '{print $1}')
if [ "$remote_sha" != %[2]q ]; then
    echo "sha256 mismatch on %[1]s: got $remote_sha expected %[2]s" >&2
    rm -f %[1]q
    exit 1
fi
gunzip -c %[1]q | docker load
rm -f %[1]q
`, remotePath, hexDigest)

	fmt.Fprintln(log, "[claude-host] verifying sha256 + docker load on agent...")
	return r.Run(ctx, script, RunOptions{Sudo: true, Stdout: log, Stderr: log})
}

// saveAndGzip runs `docker save <tag>`, gzips its stdout, and writes
// the result to outPath. Returns (sha256Digest, bytesWritten, error).
// The digest is of the gzipped output — exactly what sha256sum on the
// remote computes — so both ends can compare the same bytes.
func saveAndGzip(ctx context.Context, tag, outPath string) ([]byte, int64, error) {
	out, err := os.Create(outPath)
	if err != nil {
		return nil, 0, err
	}
	defer out.Close()

	hasher := sha256.New()
	// countingWriter gives us the file size for logging. MultiWriter
	// routes each gzip byte to the file AND the hasher AND the counter
	// in one pass — no double read of the image.
	counter := &countingWriter{w: io.MultiWriter(out, hasher)}
	gz := gzip.NewWriter(counter)

	// -n: don't write filename + mtime into the gzip header. Without it,
	// repeated saves of the same image produce different bytes (and
	// different sha256s), breaking retry semantics.
	cmd := exec.CommandContext(ctx, "docker", "save", tag)
	cmd.Stdout = gz
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return nil, 0, fmt.Errorf("docker save: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, 0, fmt.Errorf("gzip close: %w", err)
	}
	return hasher.Sum(nil), counter.n, nil
}

// countingWriter wraps a writer and tallies bytes. No lock — callers
// use it from a single goroutine.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}
