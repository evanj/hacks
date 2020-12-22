package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
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

func runLoop(pid int, runPeriod time.Duration, stopPeriod time.Duration) error {
	runLoopExiting := make(chan struct{})
	defer close(runLoopExiting)

	// create a sleep function that waits for sigint or the timeout
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, os.Interrupt)
	defer signal.Stop(sigintChan)

	sleepOrSigint := func(duration time.Duration) error {
		select {
		case <-time.After(duration):
			return nil
		case <-sigintChan:
			return errSigint
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
		err = sleepOrSigint(stopPeriod)
		err2 := unix.Kill(pid, unix.SIGCONT)
		if err != nil {
			return err
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
		err = sleepOrSigint(runPeriod)
		if err != nil {
			return err
		}

		iterationCount++
	}

	log.Printf("sent %d signals", iterationCount)
	return nil
}

func main() {
	runPeriod := flag.Duration("runPeriod", 10*time.Millisecond, "time to let process run")
	stopPeriod := flag.Duration("stopPeriod", time.Millisecond, "time to stop process with SIGSTOP")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintf(os.Stderr, "Usage: makeflaky (pid)\n")
		os.Exit(1)
	}

	pid, err := strconv.Atoi(flag.Arg(0))
	if err != nil {
		panic(err)
	}
	err = runLoop(pid, *runPeriod, *stopPeriod)
	if err != nil {
		if err == errSigint {
			log.Printf("caught SIGINT (CTRL-C)")
		} else {
			panic(err)
		}
	}
}
