package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// Debian/Ubuntu don't put postgres binaries on PATH. Find them with pg_config.
func joinPGBinPath(commandName string) (string, error) {
	configPath, err := exec.LookPath("pg_config")
	if err == nil {
		// found the pg_config process: use it to find the bin dir
		pgConfigProcess := exec.Command(configPath, "--bindir")
		out, err := pgConfigProcess.Output()
		if err != nil {
			return "", err
		}
		return filepath.Join(string(bytes.TrimSpace(out)), commandName), nil
	}
	return commandName, nil
}

func initializePostgresDir(dbDir string) error {
	// Debian/Ubuntu: initdb is not in PATH; find it with pg_config
	initDBPath, err := joinPGBinPath("initdb")
	if err != nil {
		return err
	}

	cmd := exec.Command(initDBPath, "--no-sync", "--pgdata="+dbDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type postgresProcess struct {
	proc  *exec.Cmd
	dbDir string
}

const pgSocketFileName = ".s.PGSQL.5432"

func (p *postgresProcess) socketPath() string {
	return filepath.Join(p.dbDir, pgSocketFileName)
}

func (p *postgresProcess) connectionString() string {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
	return "postgresql:///postgres?host=" + p.dbDir
}

func startPostgres(dbDir string) (*postgresProcess, error) {
	// By default Postgres puts its Unix-domain socket in /tmp; "-k ." puts it in the data dir.
	// however, then on Mac OS X we get "socket name too long" because the absolute path to the
	// socket can't exceed 100 characters
	postgresPath, err := joinPGBinPath("postgres")
	if err != nil {
		return nil, err
	}
	// -h "" means "do not listen for TCP"
	proc := exec.Command(postgresPath, "-D", dbDir, "-h", "", "-k", ".")
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	err = proc.Start()
	if err != nil {
		return nil, err
	}

	process := &postgresProcess{proc, dbDir}

	// poll for the socket to be created
	const maxPolls = 40
	// on my laptop, 15ms was SOMETIMES enough; 20ms was almost always enough time
	const pollSleep = 20 * time.Millisecond
	started := false
	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollSleep)

		_, err = os.Stat(process.socketPath())
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
		return nil, errors.New("failed to find Postgres UNIX domain socket " + process.socketPath())
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

	fmt.Printf("starting psql connection=%s ...\n", proc.connectionString())
	psql := exec.Command("psql", proc.connectionString())
	psql.Stdin = os.Stdin
	psql.Stdout = os.Stdout

	// register a sigint handler
	sigintChan := make(chan os.Signal, 1)
	signal.Notify(sigintChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigintChan)

	psqlResult := make(chan error, 1)
	go func() {
		psqlResult <- psql.Run()
	}()
	select {
	case err := <-psqlResult:
		if err != nil {
			panic(fmt.Sprintf("psql exited=%s", err.Error()))
		}

	case sig := <-sigintChan:
		fmt.Printf("postgrestmp handling signal=%s\n", sig.String())
		return
	}
}
