package postgrestest

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

// macSandboxExecOnly is a mac sandbox-exec profile that denies everything except running a process
const macSandboxExecOnly = `(version 1)
(deny default)

; Required to exec a subprocess
(allow process-exec)
(allow file-read-data)
; Required to read directory parts for a path containing /
(allow file-read-metadata)
; required by Go to auto-configure page sizes etc
(allow sysctl-read)
`

const macSandboxExecAndSysV = macSandboxExecOnly + `
; allows shmget and shmctl
(allow ipc-sysv-shm)
`

// macSandboxTestEnvVar is used to detect if the test is running as a subprocess
const macSandboxTestEnvVar = "MAC_SANDBOX_TEST_HELPER"

// macSandboxShmKeyEnvVar passes the SysV shared memory key TestMacSandboxHelper should use
const macSandboxShmKeyEnvVar = "MAC_SANDBOX_TEST_HELPER_SHM_KEY"

// Simulate the nono sandbox, to make sure we detect it correctly.
func TestDetectMacSandbox(t *testing.T) {
	sandboxExecPath, err := exec.LookPath("sandbox-exec")
	if err != nil {
		t.Skip("sandbox-exec not found in PATH: skipping (not Mac OS X?)")
	}

	if detectMacSandbox() {
		t.Skip("detectMacSandbox() returned true: skipping (probably already running in a sandbox)")
	}

	// Check that sandbox-exec works with the "exec only" policy
	execOnlyProfilePath := filepath.Join(t.TempDir(), "exec_only.sb")
	if err := os.WriteFile(execOnlyProfilePath, []byte(macSandboxExecOnly), 0o400); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command(sandboxExecPath, "-f", execOnlyProfilePath, "echo", "test").CombinedOutput()
	if err != nil {
		t.Fatalf("sandbox-exec failed: %s", err)
	}
	outStr := string(out)
	if strings.TrimSpace(outStr) != "test" {
		t.Errorf("expected output to be \"test\"; was %q", out)
	}

	// the exec only sandbox creates but can't delete the shared memory segment
	leakedKey, err := randomUnusedShmKey()
	if err != nil {
		t.Fatalf("randomUnusedShmKey failed: %s", err)
	}
	outStr, err = createShmInSandbox(sandboxExecPath, execOnlyProfilePath, leakedKey)
	if err == nil {
		t.Fatal("expected the exec only sandbox to fail; err is nil")
	}
	if !strings.Contains(outStr, "shmctl(IPC_RMID) failed: operation not permitted") {
		t.Fatalf("expected the exec only sandbox to fail to delete the shared memory segment. Output:\n%s",
			outStr)
	}

	leakedID, exists, err := findShmSegment(leakedKey)
	if err != nil {
		t.Fatalf("findShmSegment failed: %s", err)
	}
	if !exists {
		t.Fatalf("expected shared memory segment with key=%d to have leaked; it does not exist", leakedKey)
	}
	if _, err := unix.SysvShmCtl(leakedID, unix.IPC_RMID, nil); err != nil {
		t.Fatalf("failed to delete leaked shared memory segment id=%d: %s", leakedID, err)
	}

	// try the raw syscall in the sysv sandbox: it should work, and clean up after itself
	sysVProfilePath := filepath.Join(t.TempDir(), "exec_and_sysv.sb")
	if err := os.WriteFile(sysVProfilePath, []byte(macSandboxExecAndSysV), 0o400); err != nil {
		t.Fatal(err)
	}
	cleanKey, err := randomUnusedShmKey()
	if err != nil {
		t.Fatalf("randomUnusedShmKey failed: %s", err)
	}
	outStr, err = createShmInSandbox(sandboxExecPath, sysVProfilePath, cleanKey)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%#v", err)
	}
	if !strings.Contains(outStr, "PASS") {
		t.Fatalf("expected the sysv sandbox to PASS. Output:\n%s", outStr)
	}

	_, exists, err = findShmSegment(cleanKey)
	if err != nil {
		t.Fatalf("findShmSegment failed: %s", err)
	}
	if exists {
		t.Fatalf("expected shared memory segment with key=%d to have been deleted by the helper", cleanKey)
	}

	// test the sandbox detection in the exec-only sandbox: it should return true (in sandbox)
	detected, err := detectInSandbox(sandboxExecPath, execOnlyProfilePath)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%s", err)
	}
	if !detected {
		t.Fatalf("expected the exec only sandbox to be detected (detected=%t)",
			detected)
	}
	detected, err = detectInSandbox(sandboxExecPath, sysVProfilePath)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%#v", err)
	}
	if detected {
		t.Fatal("expected the sysv sandbox to be detected as not in a sandbox")
	}
}

// findShmSegment returns the ID of the SysV shared memory segment for key, and whether it exists.
func findShmSegment(key int) (int, bool, error) {
	id, err := unix.SysvShmGet(key, 0, 0)
	if err != nil {
		if errors.Is(err, syscall.ENOENT) {
			return -1, false, nil
		}
		return -1, false, err
	}
	return id, true, nil
}

// createShmInSandbox executes TestMacSandboxCreateShmHelper using sandbox-exec, to create/delete
// a SysV shared memory segment with key. It returns the raw output and error from the subprocess's
// CombinedOutput().
func createShmInSandbox(sandboxExecPath, sandboxProfilePath string, key int) (string, error) {
	cmd := exec.Command(sandboxExecPath, "-f", sandboxProfilePath,
		os.Args[0], "-test.run=TestMacSandboxCreateShmHelper")
	// pass the
	cmd.Env = append(os.Environ(), macSandboxShmKeyEnvVar+"="+strconv.Itoa(key))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Executed as a subprocess to create/delete a shared memory segment to test sandbox permissions.
func TestMacSandboxCreateShmHelper(t *testing.T) {
	shmKeyStr := os.Getenv(macSandboxShmKeyEnvVar)
	if shmKeyStr == "" {
		t.Skipf("not running as a subprocess: %s env var not set", macSandboxShmKeyEnvVar)
	}
	key, err := strconv.Atoi(shmKeyStr)
	if err != nil {
		t.Fatalf("invalid %s: %s", macSandboxShmKeyEnvVar, err)
	}

	// The Mac OS X sandbox always allows creating new shared memory segments (filed an Apple bug
	// report since this seems wrong).
	const shmSize = 56 // value used by Postgres
	id, err := unix.SysvShmGet(key, shmSize, unix.IPC_CREAT|0o600)
	if err != nil {
		t.Fatalf("shmget(..., IPC_CREAT) failed: %s", err)
	}

	// delete the shared memory segment (fails when sandboxed)
	_, err = unix.SysvShmCtl(id, unix.IPC_RMID, nil)
	if err != nil {
		t.Fatalf("shmctl(IPC_RMID) failed: %s", err)
	}
}

// detectInSandbox executes TestMacSandboxDetectHelper using sandbox-exec.
// It returns the raw output and error from the subprocess CombinedOutput().
func detectInSandbox(sandboxExecPath, sandboxProfilePath string) (bool, error) {
	cmd := exec.Command(sandboxExecPath, "-f", sandboxProfilePath,
		os.Args[0], "-test.run=TestMacSandboxDetectHelper", "-test.v")
	cmd.Env = append(os.Environ(), macSandboxTestEnvVar+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("TestMacSandboxDetectHelper failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	outStr := string(out)
	if strings.Contains(outStr, "detected=true") {
		return true, nil
	}
	if strings.Contains(outStr, "detected=false") {
		return false, nil
	}

	err = fmt.Errorf("expected \"detected=true\" or \"detected=false\"; was %q", outStr)
	return false, err
}

func TestMacSandboxDetectHelper(t *testing.T) {
	if os.Getenv(macSandboxTestEnvVar) != "1" {
		t.Skipf("not running as a test subprocess (%s env var not set)", macSandboxTestEnvVar)
	}
	detected := detectMacSandbox()
	fmt.Printf("detected=%t\n", detected)
}

func randomUnusedShmKey() (int, error) {
	const maxAttempts = 5
	for range maxAttempts {
		key := rand.Int()
		// ensure key is > 0
		key = key & (0x7FFFFFFF)
		if key == 0 {
			key = 1
		}
		if key <= 0 {
			panic(fmt.Sprintf("bug: key is %d must be > 0", key))
		}

		_, exists, err := findShmSegment(key)
		if err != nil {
			return -1, fmt.Errorf("findShmSegment failed: %s", err)
		}
		if exists {
			continue
		}
		// key is unused: success
		return key, nil
	}
	return -1, fmt.Errorf("no unused key found after %d attempts", maxAttempts)
}
