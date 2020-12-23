package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func initializePostgresDir(dbDir string) error {
	cmd := exec.Command("initdb", "-D", dbDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type postgresProcess struct {
	proc  *exec.Cmd
	dbDir string
}

const pgSocketName = "/tmp/.s.PGSQL.5432"

func startPostgres(dbDir string) (*postgresProcess, error) {
	// By default Postgres puts its Unix-domain socket in /tmp; "-k ." puts it in the data dir.
	// however, then we get "socket name too long" because the absolute path to the socket
	// can't exceed 100 characters
	proc := exec.Command("postgres", "-D", dbDir)
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	err := proc.Start()
	if err != nil {
		return nil, err
	}

	// poll for the socket to be created
	const maxPolls = 40
	// on my laptop, 15ms was SOMETIMES enough; 20ms was almost always enough time
	const pollSleep = 20 * time.Millisecond
	started := false
	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollSleep)

		_, err = os.Stat(pgSocketName)
		if err == nil {
			started = true
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if !started {
		time.Sleep(time.Hour)
		proc.Process.Kill()
		return nil, errors.New("failed to find Postgres UNIX domain socket " + pgSocketName)
	}

	return &postgresProcess{proc, dbDir}, nil
}

func (p *postgresProcess) Close() error {
	if p.proc == nil {
		return nil
	}

	proc := p.proc
	p.proc = nil

	fmt.Printf("sending postgres SIGINT ...\n")
	// SIGINT = fast shutdown: terminates all child processes
	err := proc.Process.Signal(syscall.SIGINT)
	if err != nil {
		return err
	}
	return proc.Wait()
}

func main() {
	dir, err := ioutil.TempDir("", "postgrestmp_")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)

	fmt.Printf("initializing temporary postgres database in %s ...\n", dir)
	err = initializePostgresDir(dir)
	if err != nil {
		panic(err)
	}

	fmt.Printf("starting postgres in the background ...\n")
	proc, err := startPostgres(dir)
	if err != nil {
		panic(err)
	}
	defer proc.Close()

	fmt.Printf("starting psql ...\n")
	psql := exec.Command("psql", "postgres")
	psql.Stdin = os.Stdin
	psql.Stdout = os.Stdout

	// register a sigint handler
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigintChan)

	runResult := make(chan error, 1)
	go func() {
		runResult <- psql.Run()
	}()
	select {
	case err := <-runResult:
		if err != nil {
			panic(err)
		}

	case sig := <-sigintChan:
		fmt.Printf("postgrestmp handling signal=%s\n", sig.String())
		return
	}
}
