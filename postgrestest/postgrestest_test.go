package postgrestest

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v4"
)

func TestNew(t *testing.T) {
	postgresURL := New(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, postgresURL)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping(ctx)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close(ctx)
	if err != nil {
		t.Fatal(err)
	}
}

func TestNewInstance(t *testing.T) {
	instance, err := NewInstance()
	if err != nil {
		t.Fatal(err)
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
