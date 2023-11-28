package postgrestest

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"golang.org/x/exp/slog"
)

const connectTimeout = 100 * time.Millisecond

func TestNew(t *testing.T) {
	// Postgres requires the locale to be set. on Mac OS X this fails with:
	// FATAL: postmaster became multithreaded during startup.
	// HINT: Set the LC_ALL environment variable to a valid locale.
	// This test ensures that no matter what the external LANG is set to,
	// we set it to something reasonable
	t.Setenv("LANG", "")

	postgresURL := New(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		t.Fatal(err)
	}

	// ensure the server's character encoding is UTF-8
	var serverEncoding string
	err = conn.QueryRow(ctx, `SHOW server_encoding`).Scan(&serverEncoding)
	if err != nil {
		t.Fatal(err)
	}
	if serverEncoding != "UTF8" {
		t.Errorf("expected server encoding UTF8; was %s", serverEncoding)
	}

	// Ensure we have set an ICO locale
	var upperPostgresResult string
	const lowerInput = "héllo"
	err = conn.QueryRow(ctx, `SELECT UPPER('`+lowerInput+`')`).Scan(&upperPostgresResult)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ToUpper(lowerInput) != upperPostgresResult {
		t.Errorf("expected UPPER('%s')=%s; was %s: collation error",
			lowerInput, strings.ToUpper(lowerInput), upperPostgresResult)
	}

	err = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewInstance(t *testing.T) {
	instance, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	// must not listen on localhost by default
	_, err = pgx.Connect(context.Background(), instance.LocalhostURL())
	var errno syscall.Errno
	if !(errors.As(err, &errno) && errno == syscall.ECONNREFUSED) {
		t.Errorf("connect to localhost must fail: expected err to be ECONNREFUSED, was err=%v", err)
	}

	err = instance.Close()
	if err != nil {
		t.Fatal(err)
	}
	// calling close multiple times is not an error
	err = instance.Close()
	if err != nil {
		t.Fatal(err)
	}

	// the temporary dir must be deleted
	_, err = os.Stat(instance.dbDir)
	if !os.IsNotExist(err) {
		t.Errorf("expected not exists error; err=%#v", err)
	}
}

func TestNewInstanceWithLocalhostOptions(t *testing.T) {
	instance, err := NewInstanceWithOptions(Options{ListenOnLocalhost: true})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	// must be able to connect on localhost
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, instance.LocalhostURL())
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Ping(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// must not be able to connect on other addresses
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, addr := range addrs {
		ipNetAddr := addr.(*net.IPNet)
		if ipNetAddr.IP.IsGlobalUnicast() {
			pgURL := instance.RemoteURLForAddress(ipNetAddr.IP.String())

			// Mac OS X by default firewalls all IPs so external IPs won't work but localhost will
			t.Log(pgURL)
			ctxWithTimeout, cancel := context.WithTimeout(ctx, connectTimeout)
			_, err = pgx.Connect(ctxWithTimeout, pgURL)
			cancel()
			if !(errors.Is(err, context.DeadlineExceeded) || errors.Is(err, syscall.ECONNREFUSED)) {
				t.Errorf("addr=%s ; pgURL=%s: expected ECONNREFUSED, was: %s", addr, pgURL, err)
			}
		}
	}
}

func TestNewInstanceWithGlobalOption(t *testing.T) {
	instance, err := NewInstanceWithOptions(Options{
		Logger:             slog.Default(),
		InsecureGlobalPort: 12345,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	// must be able to connect on Unix and localhost
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, instance.URL())
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Ping(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	conn, err = pgx.Connect(ctx, instance.LocalhostURL())
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Ping(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// tests that the password is included but also that RemoteURL works
	u := instance.RemoteURL()
	if !strings.Contains(u, instance.password) {
		t.Error("url should contain password")
	}

	// might be able to connect on other addresses (depends on firewalls)
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		t.Fatal(err)
	}
	for _, addr := range addrs {
		ipNetAddr := addr.(*net.IPNet)
		if ipNetAddr.IP.IsGlobalUnicast() || ipNetAddr.IP.IsLoopback() {
			pgURL := instance.RemoteURLForAddress(ipNetAddr.IP.String())

			t.Log(pgURL)
			ctxWithTimeout, cancel := context.WithTimeout(context.Background(), connectTimeout)
			conn, err := pgx.Connect(ctxWithTimeout, pgURL)
			cancel()
			if !(err == nil || errors.Is(err, context.DeadlineExceeded)) {
				t.Errorf("addr=%s ; pgURL=%s: expected success, was: %s", addr, pgURL, err)
			}
			if conn != nil {
				err = conn.Close(context.Background())
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestNewInstanceWithOptionsError(t *testing.T) {
	instance, err := NewInstanceWithOptions(Options{ListenOnLocalhost: true, InsecureGlobalPort: 12345})
	if instance != nil {
		t.Error(instance)
	}
	if !strings.Contains(err.Error(), "cannot set both") {
		t.Error(err)
	}
	instance, err = NewInstanceWithOptions(Options{InsecureGlobalPort: -1})
	if instance != nil {
		t.Error(instance)
	}
	if !strings.Contains(err.Error(), "invalid GlobalPort") {
		t.Error(err)
	}
	instance, err = NewInstanceWithOptions(Options{InsecureGlobalPort: 1 << 16})
	if instance != nil {
		t.Error(instance)
	}
	if !strings.Contains(err.Error(), "invalid GlobalPort") {
		t.Error(err)
	}
}

func TestNewInstanceWithOptionsSharedBuffers(t *testing.T) {
	instance, err := NewInstanceWithOptions(Options{SharedBuffers: 256 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, instance.URL())
	if err != nil {
		t.Fatal(err)
	}

	// ensure the shared_buffers were set from the option
	var sharedBuffers string
	err = conn.QueryRow(ctx, `SHOW shared_buffers`).Scan(&sharedBuffers)
	if err != nil {
		t.Fatal(err)
	}
	if sharedBuffers != "256MB" {
		t.Errorf("expected shared_buffers=256MB; was %s", sharedBuffers)
	}
}

func TestNewInstanceWithOptionsDirPath(t *testing.T) {
	tempDir := t.TempDir()
	pgDirPath := filepath.Join(tempDir, "pg_dir")

	options := Options{DirPath: pgDirPath}
	instance, err := NewInstanceWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	// create a table in this instance
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, instance.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, `CREATE TABLE example (column_name TEXT)`)
	if err != nil {
		t.Fatal(err)
	}
	err = conn.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = instance.Close()
	if err != nil {
		t.Fatal(err)
	}

	// reuse the directory: table must exist
	instance2, err := NewInstanceWithOptions(options)
	if err != nil {
		t.Fatal(err)
	}
	defer instance2.Close()

}
