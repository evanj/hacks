package main

import (
	"testing"
)

func TestMain(t *testing.T) {
	// ensures psql starts and exits cleanly; catches init problems on Ubuntu/Debian
	// when there are multiple versions of psql
	startPostgresAndPSQL(false, false, 0)
}
