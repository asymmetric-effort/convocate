// Package assets provides embedded build assets for the claude-shell installer.
// All files needed by the install command are compiled into the binary so that
// the binary is fully self-contained.
package assets

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"
	"io"
	"sync"
)

//go:embed data/Dockerfile.gz
var dockerfileGz []byte

//go:embed data/entrypoint.sh.gz
var entrypointGz []byte

//go:embed data/CLAUDE.md.gz
var claudeMDGz []byte

var (
	dockerfileOnce sync.Once
	dockerfileData []byte
	dockerfileErr  error

	entrypointOnce sync.Once
	entrypointData []byte
	entrypointErr  error

	claudeMDOnce sync.Once
	claudeMDData []byte
	claudeMDErr  error
)

func decompress(data []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer r.Close()
	return io.ReadAll(r)
}

// Dockerfile returns the embedded Dockerfile content.
func Dockerfile() ([]byte, error) {
	dockerfileOnce.Do(func() {
		dockerfileData, dockerfileErr = decompress(dockerfileGz)
	})
	return dockerfileData, dockerfileErr
}

// Entrypoint returns the embedded entrypoint.sh content.
func Entrypoint() ([]byte, error) {
	entrypointOnce.Do(func() {
		entrypointData, entrypointErr = decompress(entrypointGz)
	})
	return entrypointData, entrypointErr
}

// ClaudeMD returns the embedded CLAUDE.md content.
func ClaudeMD() ([]byte, error) {
	claudeMDOnce.Do(func() {
		claudeMDData, claudeMDErr = decompress(claudeMDGz)
	})
	return claudeMDData, claudeMDErr
}
