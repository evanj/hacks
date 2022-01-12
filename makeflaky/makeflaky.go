package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func isNoSuchProcess(err error) bool {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		return errno == unix.ESRCH
	}
	return false
}

var errSigint = errors.New("sigint handled")

// runLoop pauses pid using runPeriod and stopPeriod. It returns with there is an error, or if
// ctx is cancelled.
func runLoop(ctx context.Context, pid int, runPeriod time.Duration, stopPeriod time.Duration) error {
	runLoopExiting := make(chan struct{})
	defer close(runLoopExiting)

	// create a sleep function that waits for sigint or the timeout
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, os.Interrupt)
	defer signal.Stop(sigintChan)

	sleepOrSigintOrCancel := func(duration time.Duration) error {
		select {
		case <-time.After(duration):
			return nil
		case <-sigintChan:
			return errSigint
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	dutyCycle := (runPeriod.Seconds() / (runPeriod + stopPeriod).Seconds()) * 100.0
	log.Printf("slowing down pid=%d; runPeriod=%s; stopPeriod=%s (%.1f%% duty cycle)...",
		pid, runPeriod.String(), stopPeriod.String(), dutyCycle)

	iterationCount := 0
	for {
		err := unix.Kill(pid, unix.SIGSTOP)
		if err != nil {
			if isNoSuchProcess(err) {
				break
			}
			panic(err)
		}
		// make sure we always call SIGCONT, even if CTRL-C is pressed
		err = sleepOrSigintOrCancel(stopPeriod)
		err2 := unix.Kill(pid, unix.SIGCONT)
		if err != nil {
			if err == context.Canceled {
				break
			}
		}
		if err2 != nil {
			return err2
		}

		err = unix.Kill(pid, unix.SIGCONT)
		if err != nil {
			if isNoSuchProcess(err) {
				break
			}
			panic(err)
		}
		err = sleepOrSigintOrCancel(runPeriod)
		if err != nil {
			if err == context.Canceled {
				break
			}
			return err
		}

		iterationCount++
	}

	log.Printf("sent %d signals", iterationCount)
	return nil
}

// exitWithSameErr calls os.Exit with the same code as err
func exitWithSameErr(err error) {
	if err == nil {
		os.Exit(0)
	}

	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}

	panic(fmt.Sprintf("bug: err=%#v is not of type ExitError", err))
}

const usageMessage = `Usage: makeflaky (pid) || makeflaky (args) -goTest (go test args)

Pauses a process periodically to try and cause tests that depend on real time to fail.
`

func main() {
	runPeriod := flag.Duration("runPeriod", 10*time.Millisecond, "time to let process run")
	stopPeriod := flag.Duration("stopPeriod", time.Millisecond, "time to stop process with SIGSTOP")
	goTest := flag.Bool("goTest", false, "pass all other args to go test")
	goTestHelper := flag.Bool("goTestHelper", false, "pass all other args to go test")
	flag.Parse()
	if *goTest || *goTestHelper {
		if flag.NArg() == 0 {
			fmt.Fprint(os.Stderr, usageMessage)
			os.Exit(1)
		}
	} else if flag.NArg() != 1 {
		fmt.Fprint(os.Stderr, usageMessage)
		os.Exit(1)
	}

	if *goTest {
		// Run "go test -exec (self) (args)", so we can start and intercept the child process
		exePath, err := os.Executable()
		if err != nil {
			panic(err)
		}
		helperArgs := fmt.Sprintf("%s -goTestHelper -runPeriod=%s -stopPeriod=%s",
			exePath, runPeriod.String(), stopPeriod.String())
		args := []string{"test", "-exec", helperArgs}
		args = append(args, flag.Args()...)
		// fmt.Printf("goTest running: go %s ...\n", strings.Join(args, " "))
		cmd := exec.Command("go", args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		err = cmd.Run()
		exitWithSameErr(err)
	}

	var pidToPause int
	var child *exec.Cmd
	waitErrChan := make(chan error)

	ctx := context.Background()
	if *goTestHelper {
		child = exec.Command(flag.Args()[0], flag.Args()[1:]...)
		child.Stdin = os.Stdin
		child.Stdout = os.Stdout
		child.Stderr = os.Stderr
		err := child.Start()
		if err != nil {
			panic(err)
		}
		pidToPause = child.Process.Pid
		fmt.Fprintf(os.Stderr, "helper started process pid=%d %s %#v ...\n", pidToPause, child.Path, child.Args)

		// replace ctx with a context that in cancelled when the child process exits
		var cancel func()
		ctx, cancel = context.WithCancel(ctx)
		go func() {
			// ignore the error: it will be checked by the parent
			waitErr := child.Wait()
			cancel()
			waitErrChan <- waitErr
		}()
	} else {
		var err error
		pidToPause, err = strconv.Atoi(flag.Arg(0))
		if err != nil {
			panic(err)
		}
	}

	err := runLoop(ctx, pidToPause, *runPeriod, *stopPeriod)
	if err != nil {
		if err == errSigint {
			log.Printf("caught SIGINT (CTRL-C)")
		} else {
			panic(err)
		}
	}

	if child != nil {
		// get the result from wait and exit with the same code
		err = <-waitErrChan
		if err != nil {
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				os.Exit(exitErr.ExitCode())
			} else {
				panic(err)
			}
		}
	}
}
