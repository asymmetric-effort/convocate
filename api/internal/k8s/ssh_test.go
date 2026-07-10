package k8s

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestComputeCertHash_InvalidPEM(t *testing.T) {
	// Non-PEM data should still produce a hash (fallback to raw bytes)
	result := computeCertHash([]byte("not-pem-data"))
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if len(result) < 10 {
		t.Fatal("expected sha256: prefix and hex hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
}

func TestComputeCertHash_ValidPEM_InvalidCert(t *testing.T) {
	// Valid PEM structure but invalid certificate DER data
	pem := []byte("-----BEGIN CERTIFICATE-----\n" +
		base64.StdEncoding.EncodeToString([]byte("not-a-real-cert")) + "\n" +
		"-----END CERTIFICATE-----\n")
	result := computeCertHash(pem)
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
}

func TestBase64Decode(t *testing.T) {
	input := base64.StdEncoding.EncodeToString([]byte("hello world"))
	result, err := base64Decode(input)
	if err != nil {
		t.Fatalf("base64Decode: %v", err)
	}
	if string(result) != "hello world" {
		t.Fatalf("expected 'hello world', got %s", string(result))
	}
}

func TestBase64Decode_Invalid(t *testing.T) {
	_, err := base64Decode("!!!not-base64!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestBase64Decode_Empty(t *testing.T) {
	result, err := base64Decode("")
	if err != nil {
		t.Fatalf("base64Decode empty: %v", err)
	}
	if len(result) != 0 {
		t.Fatal("expected empty result for empty input")
	}
}

func TestComputeCertHash_ValidSelfSignedCert(t *testing.T) {
	// Generate a real self-signed X.509 certificate for testing
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("create cert: %v", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	result := computeCertHash(certPEM)
	if result == "" {
		t.Fatal("expected non-empty hash")
	}
	if result[:7] != "sha256:" {
		t.Fatalf("expected sha256: prefix, got %s", result[:7])
	}
	// Hash should be deterministic
	result2 := computeCertHash(certPEM)
	if result != result2 {
		t.Fatal("hash should be deterministic")
	}
}

// mockSSHExecutor records SSH calls and returns configured responses.
type mockSSHExecutor struct {
	execCalls        []sshExecCall
	execErr          error
	execWithOutCalls []sshExecCall
	execWithOutVal   string
	execWithOutErr   error
}

type sshExecCall struct {
	Host, User, Password, Script string
}

func (m *mockSSHExecutor) Exec(host, user, password, script string) error {
	m.execCalls = append(m.execCalls, sshExecCall{host, user, password, script})
	return m.execErr
}

func (m *mockSSHExecutor) ExecWithOutput(host, user, password, cmd string) (string, error) {
	m.execWithOutCalls = append(m.execWithOutCalls, sshExecCall{host, user, password, cmd})
	return m.execWithOutVal, m.execWithOutErr
}

func TestSSHExec_DelegatesToExecutor(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	err := sshExec("host1", "user1", "pass1", "echo hello")
	if err != nil {
		t.Fatalf("sshExec: %v", err)
	}
	if len(mock.execCalls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mock.execCalls))
	}
	c := mock.execCalls[0]
	if c.Host != "host1" || c.User != "user1" || c.Password != "pass1" || c.Script != "echo hello" {
		t.Fatalf("unexpected call args: %+v", c)
	}
}

func TestSSHExec_ReturnsError(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{execErr: fmt.Errorf("connection refused")}
	sshExecutor = mock

	err := sshExec("host1", "user1", "pass1", "echo hello")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecWithOutput_DelegatesToExecutor(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{execWithOutVal: "output data"}
	sshExecutor = mock

	out, err := sshExecWithOutput("host1", "user1", "pass1", "cat /etc/hostname")
	if err != nil {
		t.Fatalf("sshExecWithOutput: %v", err)
	}
	if out != "output data" {
		t.Fatalf("expected 'output data', got %s", out)
	}
}

func TestSSHExecWithOutput_ReturnsError(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{execWithOutErr: fmt.Errorf("timeout")}
	sshExecutor = mock

	_, err := sshExecWithOutput("host1", "user1", "pass1", "cmd")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecRetry_SuccessOnFirst(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{}
	sshExecutor = mock

	err := sshExecRetry("host", "user", "pass", "script", 3, time.Millisecond)
	if err != nil {
		t.Fatalf("sshExecRetry: %v", err)
	}
	if len(mock.execCalls) != 1 {
		t.Fatalf("expected 1 call on success, got %d", len(mock.execCalls))
	}
}

func TestSSHExecRetry_AllFail(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{execErr: fmt.Errorf("ssh failed")}
	sshExecutor = mock

	err := sshExecRetry("host", "user", "pass", "script", 3, time.Millisecond)
	if err == nil {
		t.Fatal("expected error after all retries")
	}
	if len(mock.execCalls) != 3 {
		t.Fatalf("expected 3 attempts, got %d", len(mock.execCalls))
	}
}

func TestSSHExecRetry_SuccessOnRetry(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	// Custom executor that fails twice then succeeds
	callCount := 0
	sshExecutor = &retryMockSSH{failUntil: 2, callCount: &callCount}

	err := sshExecRetry("host", "user", "pass", "script", 5, time.Millisecond)
	if err != nil {
		t.Fatalf("sshExecRetry: %v", err)
	}
	if callCount != 3 {
		t.Fatalf("expected 3 calls (2 fail + 1 success), got %d", callCount)
	}
}

type retryMockSSH struct {
	failUntil int
	callCount *int
}

func (r *retryMockSSH) Exec(host, user, password, script string) error {
	*r.callCount++
	if *r.callCount <= r.failUntil {
		return fmt.Errorf("attempt %d failed", *r.callCount)
	}
	return nil
}

func (r *retryMockSSH) ExecWithOutput(host, user, password, cmd string) (string, error) {
	return "", nil
}

func TestSetSSHExecutor(t *testing.T) {
	orig := sshExecutor
	defer func() { sshExecutor = orig }()

	mock := &mockSSHExecutor{}
	SetSSHExecutor(mock)

	got := GetSSHExecutor()
	if got != mock {
		t.Fatal("SetSSHExecutor/GetSSHExecutor roundtrip failed")
	}
}

func TestGetSSHExecutor(t *testing.T) {
	e := GetSSHExecutor()
	if e == nil {
		t.Fatal("expected non-nil default executor")
	}
}

func TestDefaultSSHExecutor_Exec(t *testing.T) {
	// defaultSSHExecutor.Exec delegates to sshExecReal which calls sshpass.
	// Since sshpass is not available in the test environment, we expect an error.
	d := &defaultSSHExecutor{}
	err := d.Exec("127.0.0.1:22", "nobody", "nopass", "echo test")
	// We expect an error (sshpass not found or connection refused)
	if err == nil {
		t.Fatal("expected error from defaultSSHExecutor.Exec in test environment")
	}
}

func TestDefaultSSHExecutor_ExecWithOutput(t *testing.T) {
	d := &defaultSSHExecutor{}
	_, err := d.ExecWithOutput("127.0.0.1:22", "nobody", "nopass", "echo test")
	if err == nil {
		t.Fatal("expected error from defaultSSHExecutor.ExecWithOutput in test environment")
	}
}

func TestSSHExecReal_WithPort(t *testing.T) {
	// Test the host:port branch (host already has port)
	err := sshExecReal("127.0.0.1:2222", "user", "pass", "echo hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecReal_WithoutPort(t *testing.T) {
	// Test the host-only branch (no port, defaults to :22)
	err := sshExecReal("127.0.0.1", "user", "pass", "echo hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecWithOutputReal_WithPort(t *testing.T) {
	_, err := sshExecWithOutputReal("127.0.0.1:2222", "user", "pass", "echo hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecWithOutputReal_WithoutPort(t *testing.T) {
	_, err := sshExecWithOutputReal("127.0.0.1", "user", "pass", "echo hi")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSSHExecReal_Success_WithOutput(t *testing.T) {
	origCombined := execCommandCombinedOutput
	defer func() { execCommandCombinedOutput = origCombined }()

	execCommandCombinedOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("line1\nline2\n"), nil
	}

	err := sshExecReal("10.0.0.1:22", "user", "pass", "echo hello")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestSSHExecReal_Success_NoOutput(t *testing.T) {
	origCombined := execCommandCombinedOutput
	defer func() { execCommandCombinedOutput = origCombined }()

	execCommandCombinedOutput = func(name string, args ...string) ([]byte, error) {
		return []byte{}, nil
	}

	err := sshExecReal("10.0.0.1", "user", "pass", "echo hello")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}

func TestSSHExecReal_Error_WithOutput(t *testing.T) {
	origCombined := execCommandCombinedOutput
	defer func() { execCommandCombinedOutput = origCombined }()

	execCommandCombinedOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("error output\n"), fmt.Errorf("exit status 1")
	}

	err := sshExecReal("10.0.0.1", "user", "pass", "fail")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ssh exec on 10.0.0.1") {
		t.Fatalf("expected formatted error, got: %v", err)
	}
}

func TestSSHExecWithOutputReal_Success(t *testing.T) {
	origOutput := execCommandOutput
	defer func() { execCommandOutput = origOutput }()

	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("  result data  \n"), nil
	}

	out, err := sshExecWithOutputReal("10.0.0.1:22", "user", "pass", "cmd")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if out != "result data" {
		t.Fatalf("expected 'result data', got '%s'", out)
	}
}

func TestSSHExecWithOutputReal_Error(t *testing.T) {
	origOutput := execCommandOutput
	defer func() { execCommandOutput = origOutput }()

	execCommandOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("partial"), fmt.Errorf("connection refused")
	}

	out, err := sshExecWithOutputReal("10.0.0.1", "user", "pass", "cmd")
	if err == nil {
		t.Fatal("expected error")
	}
	if out != "partial" {
		t.Fatalf("expected partial output, got '%s'", out)
	}
}

func TestSSHExecReal_OutputWithEmptyLines(t *testing.T) {
	origCombined := execCommandCombinedOutput
	defer func() { execCommandCombinedOutput = origCombined }()

	// Output with empty lines interspersed — the empty-line filter should skip them
	execCommandCombinedOutput = func(name string, args ...string) ([]byte, error) {
		return []byte("line1\n\nline2\n\n"), nil
	}

	err := sshExecReal("10.0.0.1", "user", "pass", "echo hello")
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
}
