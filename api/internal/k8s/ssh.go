package k8s

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
)

// SSHExecutor abstracts SSH command execution for testability.
type SSHExecutor interface {
	Exec(host, user, password, script string) error
	ExecWithOutput(host, user, password, cmd string) (string, error)
}

// defaultSSHExecutor uses sshpass + ssh for real SSH execution.
type defaultSSHExecutor struct{}

// sshExecutor is the active SSH implementation. Tests replace this.
var sshExecutor SSHExecutor = &defaultSSHExecutor{}

// corev1Secret is an alias to avoid import conflicts in provision.go.
type corev1Secret = corev1.Secret

func (d *defaultSSHExecutor) Exec(host, user, password, script string) error {
	return sshExecReal(host, user, password, script)
}

func (d *defaultSSHExecutor) ExecWithOutput(host, user, password, cmd string) (string, error) {
	return sshExecWithOutputReal(host, user, password, cmd)
}

// SetSSHExecutor replaces the SSH executor (for testing from external packages).
func SetSSHExecutor(e SSHExecutor) { sshExecutor = e }

// GetSSHExecutor returns the current SSH executor.
func GetSSHExecutor() SSHExecutor { return sshExecutor }

// sshExec runs a script on the remote host via the active SSHExecutor.
func sshExec(host, user, password, script string) error {
	return sshExecutor.Exec(host, user, password, script)
}

// sshExecWithOutput runs a command and returns stdout via the active SSHExecutor.
func sshExecWithOutput(host, user, password, cmd string) (string, error) {
	return sshExecutor.ExecWithOutput(host, user, password, cmd)
}

// execCommandCombinedOutput runs exec.Command and returns CombinedOutput.
// Tests replace this to avoid shelling out to real processes.
var execCommandCombinedOutput = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}

// execCommandOutput runs exec.Command and returns Output (stdout only).
// Tests replace this to avoid shelling out to real processes.
var execCommandOutput = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// sshExecReal runs a script on the remote host via sshpass + ssh.
// The API container must have openssh-client and sshpass installed.
func sshExecReal(host, user, password, script string) error {
	addr := host
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}
	hostOnly := strings.Split(addr, ":")[0]

	// Pass the entire script as a single remote command.
	// exec.Command handles argument quoting — each element is one argv entry,
	// so the script (including spaces, pipes, etc.) is a single argument to SSH.
	args := []string{
		"-p", password,
		"ssh",
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=30",
		fmt.Sprintf("%s@%s", user, hostOnly),
		script,
	}

	output, err := execCommandCombinedOutput("sshpass", args...)
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

// sshExecWithOutputReal runs a command on the remote host and returns stdout.
func sshExecWithOutputReal(host, user, password, cmd string) (string, error) {
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

	out, err := execCommandOutput("sshpass", args...)
	if err != nil {
		return string(out), fmt.Errorf("ssh exec: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// computeCertHash returns sha256:<hex> of the DER-encoded SubjectPublicKeyInfo
// from a PEM-encoded X.509 certificate, matching kubeadm's token discovery hash.
func computeCertHash(certPEM []byte) string {
	block, _ := pem.Decode(certPEM)
	if block == nil {
		// Fallback: hash the raw bytes
		h := sha256.Sum256(certPEM)
		return fmt.Sprintf("sha256:%x", h)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		h := sha256.Sum256(certPEM)
		return fmt.Sprintf("sha256:%x", h)
	}
	h := sha256.Sum256(cert.RawSubjectPublicKeyInfo)
	return fmt.Sprintf("sha256:%x", h)
}

// base64Decode decodes standard base64.
func base64Decode(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// retrySleep is the function used by sshExecRetry to wait between attempts.
// Tests replace this to avoid real delays.
var retrySleep = time.Sleep

// sshExecRetry retries sshExec up to maxAttempts times with a delay between
// attempts. This handles the case where the target VM has just booted and SSH
// isn't ready yet.
func sshExecRetry(host, user, password, script string, maxAttempts int, delay time.Duration) error {
	var lastErr error
	for i := 0; i < maxAttempts; i++ {
		lastErr = sshExec(host, user, password, script)
		if lastErr == nil {
			return nil
		}
		log.Printf("[ssh:%s] attempt %d/%d failed: %v — retrying in %v", host, i+1, maxAttempts, lastErr, delay)
		retrySleep(delay)
	}
	return lastErr
}
