// Package postgrestest creates a temporary Postgres instance. Tests should call New(). Other
// code should call NewInstance() or NewInstanceWithOptions().
package postgrestest

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/evanj/hacks/nilslog"
	"github.com/jackc/pgx/v5"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
)

// the port number must be appended to complete this
const pgSocketFileNamePrefix = ".s.PGSQL."

const pgAuthConfigFileName = "pg_hba.conf"

// LANG=C sets the "default" C locale, which is really "no locale support"
// we call initdb with arguments to use a default ICU locale with reasonable Unicode support
const langEnvVar = "LANG"
const cLocale = "C"

const defaultPort = 5432

// Options configures the Postgres instance.
type Options struct {
	// If true, Postgres will listen on localhost for network connections.
	ListenOnLocalhost bool

	// If not nil, verbose information will be logged.
	Logger *slog.Logger

	// If not 0, listen globally on InsecureGlobalPort. This is insecure because it will allow
	// connections from any IP address, although it generates and requires a password. The default
	// Postgres port is 5432. When setting InsecureGlobalPort, leave ListenOnLocalhost=false: this
	// option will listen on localhost addresses as well.
	InsecureGlobalPort int

	// Set Postgres's shared_buffers for the buffer pool cache in bytes. See:
	// https://www.postgresql.org/docs/current/runtime-config-resource.html
	SharedBuffers int

	// Create or use Postgres in this directory. If empty, it will create a temporary directory
	// that will be deleted when done. If this is set, the directory will not be deleted.
	DirPath string
}

// New creates a new Postgres instance and returns a connection string URL in the
// form "postgresql:..." to connect to using sql.Open(). After the test completes, the Postgres
// instance will be shut down. New will call t.Fatal if an error happens initializing Postgres.
// See NewInstanceWithOptions for more details.
func New(t testing.TB) string {
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
	proc       *exec.Cmd
	cfg        *pgConfig
	dbDir      string
	globalPort int
	username   string
	password   string
}

// NewInstance calls NewInstanceWithOptions() with the default options. The caller must call Close()
// to ensure it is stopped and the temporary space is deleted. Tests should prefer to call New().
func NewInstance() (*Instance, error) {
	return NewInstanceWithOptions(Options{})
}

// environWithFixedLang replaces the LANG environment variable with cLocale
func environWithFixedLang() []string {
	environ := os.Environ()
	for i, variable := range environ {
		if strings.HasPrefix(variable, langEnvVar+"=") {
			environ = slices.Delete(environ, i, i+1)
			break
		}
	}
	environ = append(environ, langEnvVar+"="+cLocale)
	return environ
}

// NewInstanceWithOptions creates a new Postgres instance in a temporary directory. The caller must
// call Close() to ensure it is stopped and the temporary space is deleted. Tests should prefer to
// call New().
//
// Postgres will use the "C" locale to ensure that tests don't depend on the local environment.
func NewInstanceWithOptions(options Options) (*Instance, error) {
	if options.ListenOnLocalhost && options.InsecureGlobalPort != 0 {
		return nil, errors.New("cannot set both ListenOnLocalhost and GlobalPort")
	}
	if options.InsecureGlobalPort < 0 || options.InsecureGlobalPort >= (1<<16) {
		return nil, fmt.Errorf("invalid GlobalPort=%d", options.InsecureGlobalPort)
	}

	options.Logger = nilslog.NewIfNil(options.Logger)

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

	cfg, err := readPGConfig(options.Logger)
	if err != nil {
		return nil, err
	}

	err = initializePostgresDir(dir, options.Logger, cfg)
	if err != nil {
		return nil, err
	}

	// add pg_hba.conf entries if needed
	if options.InsecureGlobalPort != 0 {
		f, err := os.OpenFile(filepath.Join(dir, pgAuthConfigFileName), os.O_APPEND|os.O_WRONLY, 0000)
		if err != nil {
			return nil, err
		}
		_, err = fmt.Fprintf(f, "\nhostnossl all all 0.0.0.0/0 scram-sha-256\nhostnossl all all ::0/0 scram-sha-256\n")
		if err != nil {
			return nil, err
		}
	}

	// By default Postgres puts its Unix-domain socket in /tmp; "-k ." puts it in the data dir.
	// however, then on Mac OS X we get "socket name too long" because the absolute path to the
	// socket can't exceed 100 characters
	postgresPath := cfg.binPath("postgres")

	// default for Postgres with no arguments: listen on localhost
	// -h "": do not listen for TCP
	// -h "*": listen on all addresses
	args := []string{"-D", dir, "-k", "."}
	if !options.ListenOnLocalhost {
		if options.InsecureGlobalPort == 0 {
			args = append(args, "-h", "")
		} else {
			args = append(args, "-h", "*", "-p", strconv.Itoa(options.InsecureGlobalPort))
		}
	}
	if options.SharedBuffers != 0 {
		args = append(args, "-c", fmt.Sprintf("shared_buffers=%dB", options.SharedBuffers))
	}
	proc := commandPassOutput(options.Logger, postgresPath, args...)
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

	currentUser, err := user.Current()
	if err != nil {
		return nil, err
	}
	password := ""
	if options.InsecureGlobalPort != 0 {
		passwordBytes := make([]byte, 8)
		_, err = cryptorand.Reader.Read(passwordBytes)
		if err != nil {
			return nil, err
		}
		password = hex.EncodeToString(passwordBytes)
	}

	instance := &Instance{proc, cfg, dir, options.InsecureGlobalPort, currentUser.Username, password}

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

	if options.InsecureGlobalPort != 0 {
		// this adds a pgx dependency which we already use for the test anyway
		// TODO: remove connectUntilReady and replace with pgx
		ctx := context.Background()
		conn, err := pgx.Connect(ctx, instance.LocalhostURL())
		if err != nil {
			return nil, err
		}
		defer conn.Close(ctx)
		statement := fmt.Sprintf("ALTER USER %s WITH PASSWORD $1",
			doubleQuoteIdentifier(instance.username))
		// ALTER USER needs the simple protocol for the $1 parameter replacement to work
		_, err = conn.Exec(ctx, statement,
			pgx.QueryExecModeSimpleProtocol, instance.password)
		if err != nil {
			return nil, err
		}
	}

	shouldCleanUpDir = false
	shouldKillPostgres = false
	return instance, err
}

// https://www.postgresql.org/docs/current/sql-syntax-lexical.html#SQL-SYNTAX-IDENTIFIERS
func doubleQuoteIdentifier(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// BinPath returns the absolute path to commandName in the Postgres binary directory.
func (i *Instance) BinPath(commandName string) string {
	return i.cfg.binPath(commandName)
}

func logCMD(logger *slog.Logger, cmd *exec.Cmd) {
	logger.Info("running process", "cmd_line", strings.Join(cmd.Args, " "))
}

// Debian/Ubuntu don't put postgres binaries on PATH. Find them with pg_config.
// They also wrap psql with a Perl script to allow multiple versions to co-exist.
type pgConfig struct {
	path string
}

func readPGConfig(logger *slog.Logger) (*pgConfig, error) {
	configPath, err := exec.LookPath("pg_config")
	if err != nil {
		return nil, err
	}

	// found the pg_config process: use it to find the bin dir
	pgConfigProcess := command(logger, configPath, "--bindir")
	out, err := pgConfigProcess.Output()
	if err != nil {
		return nil, err
	}
	binPath := string(bytes.TrimSpace(out))
	return &pgConfig{binPath}, nil
}

func (p *pgConfig) binPath(commandName string) string {
	return filepath.Join(p.path, commandName)
}

// command calls exec.Command and sets Env, and logs the command.
func command(logger *slog.Logger, name string, arg ...string) *exec.Cmd {
	cmd := exec.Command(name, arg...)
	cmd.Env = environWithFixedLang()
	logCMD(logger, cmd)
	return cmd
}

// commandPassOutput calls command and sets Stdout and Stderr to os.Stdout and os.Stderr.
func commandPassOutput(logger *slog.Logger, name string, arg ...string) *exec.Cmd {
	cmd := command(logger, name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd
}

func initializePostgresDir(dbDir string, logger *slog.Logger, cfg *pgConfig) error {
	// Debian/Ubuntu: initdb is not in PATH; find it with pg_config
	initDBPath := cfg.binPath("initdb")

	// --locale-provider=icu: Use ICU instead of libc for locales
	// --encoding=UTF8: Use UTF-8; don't rely on locale
	// --no-sync: return without waiting for fsync
	// --pgdata: specify cluster database
	// --username: use postgres as the superuser (I believe this changed)
	cmd := commandPassOutput(logger, initDBPath,
		"--locale-provider=icu",
		"--encoding=UTF8",
		"--icu-locale=und-x-icu",
		"--no-sync",
		"--pgdata="+dbDir)
	return cmd.Run()
}

func (i *Instance) socketPath() string {
	return filepath.Join(i.dbDir, pgSocketFileNamePrefix+strconv.Itoa(i.port()))
}

// URL returns the Postgres connection URL using a Unix socket in the form "postgresql://...". See:
// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
func (i *Instance) URL() string {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
	return "postgresql:///postgres?host=" + i.dbDir + "&port=" + strconv.Itoa(i.port())
}

func (i *Instance) port() int {
	if i.globalPort != 0 {
		return i.globalPort
	}
	return defaultPort
}

// LocalhostURL returns the Postgres connection URL using a localhost IP address. This will only
// work if using Options.ListenOnLocalhost=true. Most callers should use URL() instead.
func (i *Instance) LocalhostURL() string {
	// https://www.postgresql.org/docs/current/libpq-connect.html#LIBPQ-CONNSTRING
	return fmt.Sprintf("postgresql://127.0.0.1:%d/postgres", i.port())
}

// RemoteURL returns the first Postgres connection URL using an IP address that is not localhost.
// This will only work if using Options.InsecureGlobalPort is set. Most callers should use URL()
// instead.
func (i *Instance) RemoteURL() string {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		panic(err)
	}
	for _, address := range addresses {
		// TODO: skip other kinds of addresses?
		ipAddress := address.(*net.IPNet)
		if ipAddress.IP.IsGlobalUnicast() {
			return i.RemoteURLForAddress(ipAddress.IP.String())
		}
	}
	// TODO: log a warning?
	return i.RemoteURLForAddress("127.0.0.1")
}

func (i *Instance) RemoteURLForAddress(address string) string {
	return fmt.Sprintf("postgresql://%s:%s@%s/postgres",
		i.username, i.password, net.JoinHostPort(address, strconv.Itoa(i.port())))
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
