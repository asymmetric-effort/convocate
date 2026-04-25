package hypervisor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// DefaultUbuntuVersion is the Ubuntu release create-vm bootstraps a
// new VM from. Picked as the LTS that matches everything else the
// project provisions onto. Operators wanting a different release can
// override the URL via NewISOFetcher's option struct.
const DefaultUbuntuVersion = "22.04.5"

// IsoCacheSubdir lives under ~/.claude-shell/ so the cache survives
// across project clones / reinstalls and is operator-scoped (root
// can't race the cache).
const IsoCacheSubdir = ".claude-shell/iso"

// ISOFetcher resolves, caches, and verifies an Ubuntu live-server ISO
// on the local machine running create-vm. Path is the cached file;
// reuse across runs is gated on a sha256 match against Ubuntu's
// published SHA256SUMS file.
type ISOFetcher struct {
	// Version (e.g. "22.04.5"). Defaults to DefaultUbuntuVersion.
	Version string

	// Arch overrides the auto-detected CPU arch. "amd64" or "arm64".
	// Defaults to detectArch().
	Arch string

	// CacheDir overrides the on-disk cache. Default
	// ~/.claude-shell/iso/.
	CacheDir string

	// HTTPClient lets tests intercept downloads. Default
	// http.DefaultClient.
	HTTPClient *http.Client
}

// detectArch maps Go's runtime.GOARCH to the Ubuntu ISO naming. Calls
// out unsupported architectures explicitly so the operator gets a
// useful error instead of a 404 mid-download.
func detectArch() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "amd64", nil
	case "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("create-vm: no Ubuntu live-server ISO published for %s", runtime.GOARCH)
	}
}

// Fetch returns a local path to a verified Ubuntu live-server ISO. If
// the file is already in CacheDir and matches the expected sha256, no
// network traffic happens. Otherwise the ISO + SHA256SUMS are
// downloaded and the file's hash is verified before the path is
// returned.
func (f *ISOFetcher) Fetch() (string, error) {
	if f.Version == "" {
		f.Version = DefaultUbuntuVersion
	}
	if f.Arch == "" {
		a, err := detectArch()
		if err != nil {
			return "", err
		}
		f.Arch = a
	}
	if f.CacheDir == "" {
		home, err := userHomeDir()
		if err != nil {
			return "", err
		}
		f.CacheDir = filepath.Join(home, IsoCacheSubdir)
	}
	if f.HTTPClient == nil {
		f.HTTPClient = http.DefaultClient
	}

	if err := os.MkdirAll(f.CacheDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir %s: %w", f.CacheDir, err)
	}

	filename := fmt.Sprintf("ubuntu-%s-live-server-%s.iso", f.Version, f.Arch)
	dst := filepath.Join(f.CacheDir, filename)

	// Pull SHA256SUMS first so we know what we're checking against.
	sums, err := f.fetchSHA256SUMS()
	if err != nil {
		return "", fmt.Errorf("fetch SHA256SUMS: %w", err)
	}
	wantHex, ok := parseSHA256SUMS(sums, filename)
	if !ok {
		return "", fmt.Errorf("SHA256SUMS does not list %s", filename)
	}

	// Check existing cache.
	if existing, err := os.Stat(dst); err == nil && existing.Size() > 0 {
		got, err := sha256OfFile(dst)
		if err == nil && got == wantHex {
			return dst, nil
		}
		// Stale or corrupted — fall through to redownload.
	}

	// Download.
	if err := f.downloadFile(f.isoURL(filename), dst); err != nil {
		return "", err
	}
	got, err := sha256OfFile(dst)
	if err != nil {
		return "", err
	}
	if got != wantHex {
		_ = os.Remove(dst)
		return "", fmt.Errorf("sha256 mismatch on %s: got %s expected %s", dst, got, wantHex)
	}
	return dst, nil
}

// isoURL is the canonical download path. amd64 lives at
// releases.ubuntu.com; arm64 at cdimage.ubuntu.com — Ubuntu's
// historical split, kept stable enough that hardcoding is fine.
func (f *ISOFetcher) isoURL(filename string) string {
	if f.Arch == "arm64" {
		return fmt.Sprintf("https://cdimage.ubuntu.com/releases/%s/release/%s", baseRelease(f.Version), filename)
	}
	return fmt.Sprintf("https://releases.ubuntu.com/%s/%s", baseRelease(f.Version), filename)
}

// fetchSHA256SUMS pulls the sums manifest for the configured release.
func (f *ISOFetcher) fetchSHA256SUMS() ([]byte, error) {
	url := fmt.Sprintf("https://releases.ubuntu.com/%s/SHA256SUMS", baseRelease(f.Version))
	if f.Arch == "arm64" {
		url = fmt.Sprintf("https://cdimage.ubuntu.com/releases/%s/release/SHA256SUMS", baseRelease(f.Version))
	}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// downloadFile streams url to dst. Errors don't leave a half-written
// file in cache: write to a .partial sibling and rename on success.
func (f *ISOFetcher) downloadFile(url, dst string) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := f.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	tmp := dst + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// baseRelease drops the patch version when constructing release-tree
// URLs: "22.04.5" → "22.04". Ubuntu's directory layout is
// /releases/<major.minor>/release/.
func baseRelease(v string) string {
	if i := strings.Index(v, "."); i >= 0 {
		if j := strings.Index(v[i+1:], "."); j >= 0 {
			return v[:i+1+j]
		}
	}
	return v
}

// parseSHA256SUMS finds the line for filename in the standard
// sha256sum-formatted manifest Ubuntu publishes. Returns the lowercase
// hex digest and true on match.
//
// Manifest line shape: "<64hex> *<filename>" or "<64hex>  <filename>".
func parseSHA256SUMS(sums []byte, filename string) (string, bool) {
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		// fields[1] is the file, possibly prefixed with "*".
		name := strings.TrimPrefix(fields[1], "*")
		if name != filename {
			continue
		}
		return strings.ToLower(fields[0]), true
	}
	return "", false
}

// sha256OfFile streams path through SHA-256 and returns the lowercase
// hex digest. Used both before-download (cache validation) and
// after-download (integrity check).
var sha256OfFile = func(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// userHomeDir is exposed as a var so tests can pin the cache dir
// without depending on the runner's $HOME.
var userHomeDir = func() (string, error) {
	h, err := os.UserHomeDir()
	if err != nil {
		return "", errors.New("cannot determine home dir for ISO cache")
	}
	if h == "" {
		return "", errors.New("home dir empty")
	}
	return h, nil
}
