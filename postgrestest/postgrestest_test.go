package postgrestest

import (
	"database/sql"
	"os"
	"testing"

	"github.com/lib/pq"
)

func TestNew(t *testing.T) {
	postgresURL := New(t)
	connector, err := pq.NewConnector(postgresURL)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	if err != nil {
		t.Fatal(err)
	}
	err = db.Ping()
	if err != nil {
		t.Fatal(err)
	}
	err = db.Close()
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
