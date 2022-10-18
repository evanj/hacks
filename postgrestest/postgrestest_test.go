package postgrestest

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestNew(t *testing.T) {
	postgresURL := New(t)
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, postgresURL)
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
}

const localhostPGURL = "postgresql://127.0.0.1:5432/postgres"

func TestNewInstance(t *testing.T) {
	instance, err := NewInstance()
	if err != nil {
		t.Fatal(err)
	}
	defer instance.Close()

	// must not listen on localhost by default
	_, err = pgx.Connect(context.Background(), localhostPGURL)
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
	conn, err := pgx.Connect(ctx, localhostPGURL)
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
}
