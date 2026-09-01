package postgrestest

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

const macSandboxTestEnvVar = "MAC_SANDBOX_TEST_HELPER"

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
	if strings.TrimSpace(string(out)) != "test" {
		t.Errorf("expected output to be \"test\"; was %q", out)
	}

	// Collect all shared memory segments before the test so we can clean up others
	sharedSegmentsBefore, err := listSharedSegments()
	if err != nil {
		t.Fatalf("listSharedSegments failed: %s", err)
	}

	// the shmget syscall fails to delete the segment in the deny all sandbox
	out, err = executeHelperInSandbox(sandboxExecPath, execOnlyProfilePath)
	if err == nil {
		t.Fatal("expected the exec only sandbox to fail; err is nil")
	}
	if !strings.Contains(string(out), "shmctl(IPC_RMID) failed: operation not permitted") {
		t.Fatalf("expected the exec only sandbox to fail to delete the shared memory segment. Output:\n%s",
			string(out))
	}

	n, err := deleteNewSharedSegments(sharedSegmentsBefore)
	if err != nil {
		t.Fatalf("deleteNewSharedSegments failed: %s", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 new shared memory segment; got %d", n)
	}

	// try the raw syscall in the sysv sandbox: it should work
	sysVProfilePath := filepath.Join(t.TempDir(), "exec_and_sysv.sb")
	if err := os.WriteFile(sysVProfilePath, []byte(macSandboxExecAndSysV), 0o400); err != nil {
		t.Fatal(err)
	}
	out, err = executeHelperInSandbox(sandboxExecPath, sysVProfilePath)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%#v", err)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Fatalf("expected the sysv sandbox to PASS. Output:\n%s",
			string(out))
	}

	n, err = deleteNewSharedSegments(sharedSegmentsBefore)
	if err != nil {
		t.Fatalf("deleteNewSharedSegments failed: %s", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 new shared memory segments; got %d", n)
	}

	// test the sandbox detection in the exec-only sandbox: it should return true (in sandbox)
	detected, err := executeDetectInSandbox(t, sandboxExecPath, execOnlyProfilePath)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%s", err)
	}
	if !detected {
		t.Fatalf("expected the exec only sandbox to be detected (detected=%t)",
			detected)
	}
	detected, err = executeDetectInSandbox(t, sandboxExecPath, sysVProfilePath)
	if err != nil {
		t.Fatalf("expected the sysv sandbox to succeed; err=%#v", err)
	}
	if detected {
		t.Fatal("expected the sysv sandbox to be detected as not in a sandbox")
	}

	sharedSegmentsEnd, err := listSharedSegments()
	if err != nil {
		t.Fatalf("listSharedSegments failed: %s", err)
	}
	if !slices.Equal(sharedSegmentsEnd, sharedSegmentsBefore) {
		t.Fatalf("deleteNewSharedSegments did not delete the expected shared memory segments\nbefore=%#v\nend=%#v",
			sharedSegmentsBefore, sharedSegmentsEnd)
	}
}

// listSharedSegments runs `ipcs -m` and returns the shared memory segment IDs that exist.
func listSharedSegments() ([]int, error) {
	out, err := exec.Command("ipcs", "-m").Output()
	if err != nil {
		return nil, err
	}

	var ids []int
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		// data rows look like: m <id> <key> <mode> <owner> <group>
		if len(fields) < 2 || fields[0] != "m" {
			continue
		}
		id, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return ids, nil
}

// deleteNewSharedSegments deletes the shared memory segments that are not included in exclude.
// It returns the number of segments deleted.
func deleteNewSharedSegments(exclude []int) (int, error) {
	excludeSet := make(map[int]bool, len(exclude))
	for _, id := range exclude {
		excludeSet[id] = true
	}

	ids, err := listSharedSegments()
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, id := range ids {
		if excludeSet[id] {
			continue
		}
		out, err := exec.Command("ipcrm", "-m", strconv.Itoa(id)).CombinedOutput()
		if err != nil {
			return deleted, fmt.Errorf("ipcrm -m %d failed: %w: %s", id, err, strings.TrimSpace(string(out)))
		}
		deleted++
	}
	return deleted, nil
}

// executeHelperInSandbox executes TestMacSandboxHelper using sandbox-exec and a subprocess.
// It returns the raw output and error from the subprocess CombinedOutput().
func executeHelperInSandbox(sandboxExecPath, sandboxProfilePath string) ([]byte, error) {
	// cmd := exec.Command(os.Args[0], "-test.run=TestMacSandboxHelper")
	cmd := exec.Command(sandboxExecPath, "-f", sandboxProfilePath,
		os.Args[0], "-test.run=TestMacSandboxHelper")
	// set the environment variable so the helper can detect that it is being run by the test
	cmd.Env = append(os.Environ(), macSandboxTestEnvVar+"=1")
	return cmd.CombinedOutput()
}

// Helper function executed as a subprocess
func TestMacSandboxHelper(t *testing.T) {
	if os.Getenv(macSandboxTestEnvVar) != "1" {
		t.Skip("not running as a sandbox helper: skipping")
	}

	key, err := randomUnusedShmKey()
	if err != nil {
		t.Fatalf("randomUnusedShmKey failed: %s", err)
	}

	// create the shared memory segment (expected to always work)
	const shmSize = 56 // arbitrary; matches shmtest.c
	id, err := unix.SysvShmGet(key, shmSize, unix.IPC_CREAT|0o600)
	if err != nil {
		t.Fatalf("shmget failed: %s", err)
	}

	// delete the shared memory segment (fails when sandboxed)
	_, err = unix.SysvShmCtl(id, unix.IPC_RMID, nil)
	if err != nil {
		t.Fatalf("shmctl(IPC_RMID) failed: %s", err)
	}
}

// executeDetectInSandbox executes TestMacSandboxDetectHelper using sandbox-exec and a subprocess.
// It returns the raw output and error from the subprocess CombinedOutput().
func executeDetectInSandbox(t *testing.T, sandboxExecPath, sandboxProfilePath string) (bool, error) {
	cmd := exec.Command(sandboxExecPath, "-f", sandboxProfilePath,
		os.Args[0], "-test.run=TestMacSandboxDetectHelper", "-test.v")
	cmd.Env = append(os.Environ(), macSandboxTestEnvVar+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("TestMacSandboxDetectHelper failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if strings.Contains(string(out), "detected=true") {
		return true, nil
	}
	if strings.Contains(string(out), "detected=false") {
		return false, nil
	}

	return false, fmt.Errorf("expected output to contain \"detected=true\" or \"detected=false\"; was %q", string(out))
}

func TestMacSandboxDetectHelper(t *testing.T) {
	if os.Getenv(macSandboxTestEnvVar) != "1" {
		t.Skip("not running as a sandbox helper: skipping")
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

		_, err := unix.SysvShmGet(key, 0, 0)
		if err != nil {
			if errors.Is(err, syscall.ENOENT) {
				// key is unused: success
				return key, nil
			}
			return -1, fmt.Errorf("shmget failed: %s", err)
		}
	}
	return -1, fmt.Errorf("no unused key found after %d attempts", maxAttempts)
}
