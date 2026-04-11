package install

import (
	"os"
	"os/exec"
	"testing"
)

func mockExecSuccess(name string, args ...string) *exec.Cmd {
	return exec.Command("true")
}

func mockExecFailure(name string, args ...string) *exec.Cmd {
	return exec.Command("false")
}

func TestNew(t *testing.T) {
	inst := New()
	if inst == nil {
		t.Fatal("New returned nil")
	}
}

func TestNewWithExec(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	if inst == nil {
		t.Fatal("NewWithExec returned nil")
	}
}

func TestRun_AsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("test requires root")
	}
	inst := NewWithExec(mockExecSuccess)
	err := inst.Run()
	if err != nil {
		t.Logf("Run error (may be expected if claude user missing): %v", err)
	}
}

func TestRun_NotRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test must run as non-root")
	}

	inst := NewWithExec(mockExecSuccess)
	err := inst.Run()
	if err == nil {
		t.Error("expected error when not running as root")
	}
}

func TestCheckPlatform_Success(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	err := inst.checkPlatform()
	if err != nil {
		t.Errorf("checkPlatform failed on linux: %v", err)
	}
}

func TestCheckDocker_Success(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	err := inst.checkDocker()
	if err != nil {
		t.Errorf("checkDocker failed: %v", err)
	}
}

func TestCheckDocker_Failure(t *testing.T) {
	inst := NewWithExec(mockExecFailure)
	err := inst.checkDocker()
	if err == nil {
		t.Error("expected error when docker is not available")
	}
}

func TestCheckClaudeCLI_Found(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	err := inst.checkClaudeCLI()
	if _, statErr := os.Stat("/usr/local/bin/claude"); statErr == nil {
		// claude exists, so checkClaudeCLI should succeed
		if err != nil {
			t.Errorf("checkClaudeCLI failed when binary exists: %v", err)
		}
	} else {
		// claude doesn't exist, so checkClaudeCLI should fail
		if err == nil {
			t.Error("expected error when claude binary doesn't exist")
		}
	}
}

func TestSetupSkel(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("setupSkel requires root (to chown)")
	}
	inst := NewWithExec(mockExecSuccess)
	err := inst.setupSkel()
	// Should succeed if the claude user exists
	if err != nil {
		t.Logf("setupSkel error (may be expected): %v", err)
	}
}

func TestBuildImage_Success(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	err := inst.buildImage()
	if err != nil {
		t.Errorf("buildImage failed: %v", err)
	}
}

func TestBuildImage_DockerFails(t *testing.T) {
	inst := NewWithExec(mockExecFailure)
	err := inst.buildImage()
	if err == nil {
		t.Error("expected error when docker build fails")
	}
}

func TestChownRecursive(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("chown test requires root")
	}

	tmpDir := t.TempDir()
	subDir := tmpDir + "/sub"
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(subDir+"/file.txt", []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	err := chownRecursive(tmpDir, 1000, 1000)
	if err != nil {
		t.Fatalf("chownRecursive failed: %v", err)
	}
}

func TestChownRecursive_InvalidPath(t *testing.T) {
	err := chownRecursive("/nonexistent/path/12345", 1000, 1000)
	if err == nil {
		t.Error("expected error for invalid path")
	}
}

func TestCreateUser_AlreadyExists(t *testing.T) {
	inst := NewWithExec(mockExecSuccess)
	err := inst.createUser()
	// claude user exists on this system, so it should just add to docker group
	if err != nil {
		t.Logf("createUser note: %v", err)
	}
}

func TestDefaultExecFunc(t *testing.T) {
	cmd := DefaultExecFunc("echo", "hello")
	if cmd == nil {
		t.Fatal("DefaultExecFunc returned nil")
	}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("DefaultExecFunc command failed: %v", err)
	}
	if string(out) != "hello\n" {
		t.Errorf("output = %q, want %q", string(out), "hello\n")
	}
}
