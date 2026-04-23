package hostinstall

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/term"
)

// SSHConfig is the set of knobs NewSSHRunner needs to dial a target.
type SSHConfig struct {
	Host string // hostname or IP
	Port int    // default 22 when zero
	User string // remote user; must have NOPASSWD sudo on the target

	// Timeout applies to the TCP dial + handshake. Defaults to 15s when zero.
	Timeout time.Duration

	// PasswordPrompt is called when key/agent auth fails. It should return the
	// SSH password for the given user@host, or an error to abort. When nil,
	// DefaultPasswordPrompt is used.
	PasswordPrompt func(user, host string) (string, error)

	// HostKeyCallback verifies the remote's host key. When nil,
	// ssh.InsecureIgnoreHostKey is used — acceptable for interactive
	// first-time provisioning but noisy for long-lived automation.
	HostKeyCallback ssh.HostKeyCallback
}

// SSHRunner executes commands on a remote host via SSH.
type SSHRunner struct {
	client *ssh.Client
	cfg    SSHConfig
}

// NewSSHRunner dials the target in cfg and returns a connected runner. It
// tries (in order): the SSH agent, ~/.ssh/id_{ed25519,rsa,ecdsa}, and finally
// falls back to PasswordPrompt.
func NewSSHRunner(cfg SSHConfig) (*SSHRunner, error) {
	if cfg.Host == "" {
		return nil, fmt.Errorf("ssh: host required")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.User == "" {
		cfg.User = os.Getenv("USER")
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 15 * time.Second
	}
	if cfg.PasswordPrompt == nil {
		cfg.PasswordPrompt = DefaultPasswordPrompt
	}
	hostKeyCB := cfg.HostKeyCallback
	if hostKeyCB == nil {
		hostKeyCB = ssh.InsecureIgnoreHostKey()
	}

	auths := collectAuthMethods(cfg)
	clientCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auths,
		HostKeyCallback: hostKeyCB,
		Timeout:         cfg.Timeout,
	}

	addr := net.JoinHostPort(cfg.Host, fmt.Sprintf("%d", cfg.Port))
	client, err := ssh.Dial("tcp", addr, clientCfg)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	return &SSHRunner{client: client, cfg: cfg}, nil
}

// Target implements Runner.
func (r *SSHRunner) Target() string {
	return fmt.Sprintf("%s@%s", r.cfg.User, r.cfg.Host)
}

// Close implements Runner.
func (r *SSHRunner) Close() error {
	if r.client == nil {
		return nil
	}
	return r.client.Close()
}

// Run implements Runner. When opts.Sudo is true the command is wrapped with
// `sudo -n --` so NOPASSWD sudo is required on the remote.
func (r *SSHRunner) Run(ctx context.Context, cmd string, opts RunOptions) error {
	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh new session: %w", err)
	}
	defer session.Close()

	// Hook up IO streams.
	if opts.Stdout != nil {
		session.Stdout = opts.Stdout
	}
	if opts.Stderr != nil {
		session.Stderr = opts.Stderr
	}
	if opts.Stdin != nil {
		session.Stdin = opts.Stdin
	}
	for _, kv := range opts.Env {
		if i := strings.IndexByte(kv, '='); i > 0 {
			_ = session.Setenv(kv[:i], kv[i+1:])
		}
	}

	remoteCmd := cmd
	if opts.Sudo {
		// Use bash -c to preserve the command verbatim even with pipes/quotes.
		remoteCmd = "sudo -n -- bash -c " + shellQuote(cmd)
	}

	done := make(chan error, 1)
	go func() { done <- session.Run(remoteCmd) }()
	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("remote %s: %w", r.Target(), err)
		}
	}
	return nil
}

// CopyFile implements Runner by streaming the local file through `cat >
// destPath && chmod` on the remote. This avoids a hard dependency on `scp`
// being present on the target.
func (r *SSHRunner) CopyFile(ctx context.Context, srcPath, destPath string, mode os.FileMode) error {
	src, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("open %s: %w", srcPath, err)
	}
	defer src.Close()

	session, err := r.client.NewSession()
	if err != nil {
		return fmt.Errorf("ssh new session: %w", err)
	}
	defer session.Close()

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}

	tmp := destPath + ".tmp"
	// Use install(1) on the remote to atomically place the file with correct
	// permissions. install creates intermediate dirs with -D.
	remoteCmd := fmt.Sprintf("sudo -n -- install -D -m %o /dev/stdin %s && sudo -n -- mv %s %s",
		mode, shellQuote(tmp), shellQuote(tmp), shellQuote(destPath))
	if err := session.Start(remoteCmd); err != nil {
		return fmt.Errorf("start remote copy: %w", err)
	}
	if _, err := io.Copy(stdin, src); err != nil {
		_ = stdin.Close()
		return fmt.Errorf("stream bytes: %w", err)
	}
	if err := stdin.Close(); err != nil {
		return fmt.Errorf("close stdin: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- session.Wait() }()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		if err != nil {
			return fmt.Errorf("remote copy to %s: %w", destPath, err)
		}
	}
	return nil
}

// collectAuthMethods prefers ssh-agent, then default identity files, then a
// password prompt fallback.
func collectAuthMethods(cfg SSHConfig) []ssh.AuthMethod {
	var methods []ssh.AuthMethod

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			a := agent.NewClient(conn)
			methods = append(methods, ssh.PublicKeysCallback(a.Signers))
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		for _, name := range []string{"id_ed25519", "id_rsa", "id_ecdsa"} {
			p := filepath.Join(home, ".ssh", name)
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			signer, err := ssh.ParsePrivateKey(data)
			if err != nil {
				continue
			}
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	// Password fallback (interactive).
	methods = append(methods, ssh.PasswordCallback(func() (string, error) {
		return cfg.PasswordPrompt(cfg.User, cfg.Host)
	}))
	return methods
}

// DefaultPasswordPrompt reads a password from the controlling terminal
// without echoing. Returns an error if the process has no TTY.
var DefaultPasswordPrompt = func(user, host string) (string, error) {
	fmt.Fprintf(os.Stderr, "password for %s@%s: ", user, host)
	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		return "", fmt.Errorf("cannot prompt for password: stdin is not a terminal")
	}
	pw, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(pw), nil
}

// shellQuote wraps s in single quotes, escaping any single quotes inside.
// Safe for passing arbitrary strings through `bash -c`.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}
