package k8s

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// corev1Secret is an alias to avoid import conflicts in provision.go.
type corev1Secret = corev1.Secret

// sshExec runs a script on the remote host via sshpass + ssh.
// The API container must have openssh-client and sshpass installed.
func sshExec(host, user, password, script string) error {
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}
	hostOnly := strings.Split(addr, ":")[0]

	args := []string{
		"-p", password,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", user, hostOnly),
		"bash", "-c", script,
	}

	cmd := exec.Command("sshpass", args...)
	output, err := cmd.CombinedOutput()
	if len(output) > 0 {
		for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
			if line != "" {
				log.Printf("[ssh:%s] %s", hostOnly, line)
			}
		}
	}
	if err != nil {
		return fmt.Errorf("ssh exec on %s: %w\noutput: %s", hostOnly, err, string(output))
	}
	return nil
}

// sshExecWithOutput runs a command on the remote host and returns stdout.
func sshExecWithOutput(host, user, password, cmd string) (string, error) {
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}
	hostOnly := strings.Split(addr, ":")[0]

	args := []string{
		"-p", password,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		fmt.Sprintf("%s@%s", user, hostOnly),
		cmd,
	}

	c := exec.Command("sshpass", args...)
	out, err := c.Output()
	if err != nil {
		return string(out), fmt.Errorf("ssh exec: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// computeCertHash returns sha256:<hex> of the given certificate bytes.
func computeCertHash(certData []byte) string {
	h := sha256.Sum256(certData)
	return fmt.Sprintf("sha256:%x", h)
}

// base64Decode decodes standard base64.
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// unused but kept for interface completeness
var _ = time.Second
