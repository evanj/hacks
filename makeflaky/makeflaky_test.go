package main

import (
	"errors"
	"flag"
	"log"
	"os"
	"testing"
	"time"
)

const childSleep = 50 * time.Millisecond

func TestMain(m *testing.M) {
	runTests := flag.Bool("runTests", true, "Set to false to not run tests")
	exitCode := flag.Int("exitCode", 0, "Exit code if runTests is false")
	flag.Parse()

	if *runTests {
		os.Exit(m.Run())
	}

	log.Printf("test child process sleeping for %s then exiting with code %d ...",
		childSleep, *exitCode)
	time.Sleep(childSleep)
	os.Exit(*exitCode)
}

func TestExecAndExitCode(t *testing.T) {
	testExePath, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()

	const execFlag = true
	const runPeriod = 10 * time.Millisecond
	const stopPeriod = runPeriod
	args := []string{testExePath, "-runTests=false", "-exitCode=42"}
	err = maybeExecAndRun(execFlag, runPeriod, stopPeriod, args)

	end := time.Now()

	if exitCode(err) != 42 {
		t.Errorf("expected exit code 42 was %d", exitCode(err))
	}
	elapsed := end.Sub(start)
	if elapsed < childSleep {
		t.Errorf("elapsed=%s < childSleep=%s", elapsed, childSleep)
	}
}

func TestExitCodeNotExitErr(t *testing.T) {
	// an error that is not exec.ExitError returns code 1
	notExitErr := errors.New("test error")
	code := exitCode(notExitErr)
	const expected = 1
	if code != expected {
		t.Errorf("exitCode(%#v)=%d; expected %d", notExitErr, code, expected)
	}

	if exitCode(nil) != 0 {
		t.Errorf("exitCode(0)=%d; expected 0", exitCode(nil))
	}
}
