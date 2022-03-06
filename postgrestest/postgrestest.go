// Package postgrestest allows creating a temporary Postgres instance for the duration of a Go test
// or package.
package postgrestest

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// New creates a new Postgres instance and returns a connection string URL in the
// form "postgresql:..." to connect to using sql.Open(). After the test completes, the Postgres
// instance will be shut down. New will call t.Fatal if an error happens initializing Postgres.
func New(t *testing.T) string {
	instance, err := NewInstance()
	if err != nil {
		t.Fatalf("failed starting postgres: %s", err.Error())
		// Fatalf should terminate execution, but just in case, don't return "": it is a valid string!
		return "invalid_connection_string"
	}
	t.Cleanup(func() {
		err := instance.Close()
		if err != nil {
			t.Logf("warning: error shutting down Postgres: %s", err.Error())
		}
	})
	return instance.URL()
}

// Instance contains the state of a new temporary Postgres instance.
type Instance struct {
	proc  *exec.Cmd
	dbDir string
}

// NewInstance creates a new Postgres instance in a temporary directory. The caller must call
// Close() on it to ensure it is stopped and the temporary space is deleted. Most callers should
// use NewPostgresForTest(t) instead.
func NewInstance() (*Instance, error) {
	shouldCleanUpDir := true
	dir, err := os.MkdirTemp("", "postgrestest_")
	if err != nil {
		return nil, err
	}
	defer func() {
		if shouldCleanUpDir {
			os.RemoveAll(dir)
		}
	}()

	// TODO: refactor initializePostgresDir to avoid calling joinPGBinPath TWICE
	err = initializePostgresDir(dir)
	if err != nil {
		return nil, err
	}

	// By default Postgres puts its Unix-domain socket in /tmp; "-k ." puts it in the data dir.
	// however, then on Mac OS X we get "socket name too long" because the absolute path to the
	// socket can't exceed 100 characters
	postgresPath, err := joinPGBinPath("postgres")
	if err != nil {
		return nil, err
	}
	// -h "" means "do not listen for TCP"
	// TODO: Add tuning parameters? E.g. -c shared_buffers='1G'?
	proc := exec.Command(postgresPath, "-D", dir, "-h", "", "-k", ".")
	// TODO: capture output somewhere else?
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr
	err = proc.Start()
	if err != nil {
		return nil, err
	}

	shouldKillPostgres := true
	defer func() {
		if shouldKillPostgres {
			proc.Process.Kill()
		}
	}()

	instance := &Instance{proc, dir}

	// poll for the socket to be created
	const maxPolls = 40
	const pollSleep = 10 * time.Millisecond
	started := false
	for i := 0; i < maxPolls; i++ {
		time.Sleep(pollSleep)

		_, err = os.Stat(instance.socketPath())
		if err == nil {
			started = true
			break
		}
		if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if !started {
		return nil, errors.New("postgrestest: failed to find UNIX socket: " + instance.socketPath())
	}

	// poll until the server is not failing connections with "starting up"
	err = connectUntilReady(instance.socketPath())
	if err != nil {
		return nil, err
	}

	shouldCleanUpDir = false
	shouldKillPostgres = false
	return instance, err
}

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

const pgSocketFileName = ".s.PGSQL.5432"

func (i *Instance) socketPath() string {
	return filepath.Join(i.dbDir, pgSocketFileName)
}

// URL returns the Postgres connection URL in the form "postgresql://...". See:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func (i *Instance) URL() string {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
	return "postgresql:///postgres?host=" + i.dbDir
}

// Close shuts down Postgres and deletes the temporary directory.
func (i *Instance) Close() error {
	// allow calling Close() multiple times
	if i.proc == nil {
		return nil
	}
	proc := i.proc
	i.proc = nil

	// SIGQUIT = immediate shutdown: terminates all child processes and sends kill within 5 seconds
	// https://www.postgresql.org/docs/14/server-shutdown.html
	err := proc.Process.Signal(syscall.SIGQUIT)
	if err != nil {
		return err
	}
	err = proc.Wait()
	err2 := os.RemoveAll(i.dbDir)
	if err != nil {
		return err
	}
	return err2
}

// See cannot_connect_now and ERRCODE_CANNOT_CONNECT_NOW:
// https://www.postgresql.org/docs/14/errcodes-appendix.html
// https://github.com/postgres/postgres/blob/master/src/backend/postmaster/postmaster.c
//
// Encoded as ErrorResponse field type 'C' SQLSTATE code followed by NUL terminated string
const cannotConnectErrCode = "C57P03\x00"

const msgErrKind = 'E'

// connectUntilReady polls the socket until it stops returning "the database system is starting up"
func connectUntilReady(unixSocketPath string) error {
	// this does this the hard way to avoid direct dependencies on DB drivers
	// this is probably stupid, but means users can use whatever driver they want, or none at all

	// connect to socket and see if the DB is ready
	const maxAttempts = 40
	const attemptSleep = 10 * time.Millisecond
	for i := 0; i < maxAttempts; i++ {
		// Connect to the socket
		clientConn, err := net.Dial("unix", unixSocketPath)
		if err != nil {
			return err
		}
		err = writeStartupMessage(clientConn)
		if err != nil {
			return err
		}

		// read the response
		msg, err := readMessage(clientConn)
		if err != nil {
			return err
		}

		// close the connection
		err = clientConn.Close()
		if err != nil {
			return err
		}

		if msg.kind == msgErrKind && bytes.Contains(msg.body, []byte(cannotConnectErrCode)) {
			// this is the "cannot connect" error: wait and try again
			time.Sleep(attemptSleep)
			continue
		}

		// some other response! Assume success, or the driver will report the error
		return nil
	}

	// we are still getting "cannot connect": report the error elsewhere
	return nil
}

func writeStartupMessage(w io.Writer) error {
	// See StartupMessage
	// https://www.postgresql.org/docs/14/protocol-message-formats.html
	// StartupMessage

	var msg bytes.Buffer
	const protocolMajor = 3
	const protocolMinor = 0
	protocol := int32(protocolMajor<<16 | protocolMinor)
	writeI32(&msg, protocol)

	// write the key/value parameters: user is required
	writeString(&msg, "user")
	writeString(&msg, "postgres")

	// "A zero byte is required as a terminator after the last name/value pair"
	msg.WriteByte(0x00)

	// add the length prefix in front of this message
	msgLen := msg.Len() + 4
	var outerMsg bytes.Buffer
	writeI32(&outerMsg, int32(msgLen))
	_, err := msg.WriteTo(&outerMsg)
	if err != nil {
		panic(err)
	}

	// write the entire packet
	_, err = outerMsg.WriteTo(w)
	return err
}

type message struct {
	kind byte
	body []byte
}

// readMessage
func readMessage(r io.Reader) (*message, error) {
	// read the kind and length
	buf := make([]byte, 1024)
	_, err := io.ReadFull(r, buf[:5])
	if err != nil {
		return nil, err
	}

	kind := buf[0]
	// msgLen includes self
	msgLen := int32(binary.BigEndian.Uint32(buf[1:5])) - 4
	if msgLen < 0 || int(msgLen) > len(buf) {
		return nil, fmt.Errorf("msgLen=%d out of bounds", msgLen)
	}
	_, err = io.ReadFull(r, buf[:msgLen])
	if err != nil {
		return nil, err
	}
	return &message{kind, buf[:msgLen]}, nil
}

func writeI32(w io.Writer, v int32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], uint32(v))
	_, err := w.Write(buf[:])
	if err != nil {
		panic(err)
	}
}

func writeString(w io.Writer, s string) {
	// null terminated C string
	_, err := w.Write([]byte(s))
	if err != nil {
		panic(err)
	}
	_, err = w.Write([]byte{0x00})
	if err != nil {
		panic(err)
	}
}

// https://www.postgresql.org/docs/14/protocol-flow.html#id-1.10.5.7.3