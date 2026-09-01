//go:build darwin

package postgrestest

/*
#cgo CFLAGS: -std=c17 -Wall -Wextra
#include "detect_sandbox_darwin.h"
*/
import "C"

// detectMacSandbox returns true if the process is running inside a Mac OS X sandbox that prevents
// Postgres from working. Notably: the nono sandbox for LLM agents uses sandbox-exec to forbid
// shmget. When blocked shmget can create a new shared memory segment, but can't delete it or attach
// to it. Postgres uses shmget to detect other processes on the same data directory. This means
// calling initdb creates a shared memory segment, but it can't delete it. This leaks shared memory
// segments, until the system hits its limit. At that point, it will return ENOSPC (no space left on
// device). The system is then stuck until the shared memory segments are manually deleted or the
// system is rebooted.
//
// To prevent agents from getting the system stuck, we detect the sandbox then refuse to start
// Postgres. To detect it, we need to use some horrible private APIs, since otherwise our test
// would also leak shared memory segments.
func detectMacSandbox() bool {
	return bool(C.detect_mac_sandbox())
}
