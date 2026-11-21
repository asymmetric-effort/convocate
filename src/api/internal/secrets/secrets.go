package secrets

import (
	"os"
	"strings"
)

// ReadToken reads a secret from a file when fileEnvVar names a path, or
// falls back to tokenEnvVar. The file is preferred because it can be
// mounted with 0400 permissions, keeping the secret out of the process
// environment.
func ReadToken(fileEnvVar, tokenEnvVar string) string {
	if path := os.Getenv(fileEnvVar); path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	return os.Getenv(tokenEnvVar)
}
