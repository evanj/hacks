//go:build !darwin

package postgrestest

// detectMacSandbox returns true if the process is running inside a Mac OS X sandbox that prevents
// Postgres from working. It always returns false on non Mac OS X systems. See the version
// in detect_sandbox_darwin.go for details.
func detectMacSandbox() bool {
	return false
}
