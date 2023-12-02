package pgxtxn

import (
	"context"
	"fmt"
	"testing"

	"github.com/evanj/hacks/postgrestest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestRetryDeadlock(t *testing.T) {
	pgURL := postgrestest.New(t)
	pgPool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err = pgPool.Exec(ctx, "CREATE TABLE conflict_example (id INT NOT NULL PRIMARY KEY, value TEXT NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pgPool.Exec(ctx, `INSERT INTO conflict_example (id, value) VALUES (1, ''), (2, '')`)
	if err != nil {
		t.Fatal(err)
	}

	errs := make(chan error)
	txnsDidFirstModification := make(chan struct{}) // must be unbuffered

	// first transaction: updates row 1 then signals that it is done
	firstCount := 0
	firstTxnBody := func(ctx context.Context, tx pgx.Tx) error {
		firstCount++
		_, err := tx.Exec(ctx, `UPDATE conflict_example SET value = 'first' WHERE id = 1`)
		if err != nil {
			return err
		}

		// have the two transactions wait for each other
		if firstCount == 1 {
			txnsDidFirstModification <- struct{}{}
		}

		// modify the other row to cause a deadlock
		_, err = tx.Exec(ctx, `UPDATE conflict_example SET value = 'first' WHERE id = 2`)
		return err
	}
	go func() {
		errs <- Run(ctx, pgPool, firstTxnBody, pgx.TxOptions{})
	}()

	secondCount := 0
	// second transaction: wait for first update then do our own update
	secondTxnBody := func(ctx context.Context, tx pgx.Tx) error {
		secondCount++
		_, err := tx.Exec(ctx, `UPDATE conflict_example SET value = 'second' WHERE id = 2`)
		if err != nil {
			return err
		}

		// have the two transactions wait for each other
		if secondCount == 1 {
			<-txnsDidFirstModification
		}

		// modify the other row to cause a deadlock
		_, err = tx.Exec(ctx, `UPDATE conflict_example SET value = 'second' WHERE id = 1`)
		return err
	}
	go func() {
		errs <- Run(ctx, pgPool, secondTxnBody, pgx.TxOptions{})
	}()

	const numGoroutines = 2
	for i := 0; i < numGoroutines; i++ {
		err = <-errs
		if err != nil {
			t.Fatal(err)
		}
	}
	if firstCount+secondCount != 3 {
		t.Errorf("firstCount: %d, secondCount: %d", firstCount, secondCount)
	}
}

func TestRetrySerializationFailure(t *testing.T) {
	pgURL := postgrestest.New(t)
	pgPool, err := pgxpool.New(context.Background(), pgURL)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	_, err = pgPool.Exec(ctx, "CREATE TABLE conflict_example (id INT NOT NULL PRIMARY KEY, value TEXT NOT NULL)")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pgPool.Exec(ctx, `INSERT INTO conflict_example (id, value) VALUES (1, 'a')`)
	if err != nil {
		t.Fatal(err)
	}

	// transaction: updates row 1 then signals that it is done
	count := 0
	firstTxnBody := func(ctx context.Context, tx pgx.Tx) error {
		count++
		row := tx.QueryRow(ctx, `SELECT value FROM conflict_example WHERE id = 1`)
		var value string
		err = row.Scan(&value)
		if err != nil {
			return err
		}
		if value != "a" && value != "second_tx" {
			return fmt.Errorf("unexpected value: %#v", value)
		}
		if count == 1 {
			t.Logf("first txn queried value=%#v; doing non-transactional update ...", value)
			_, err := pgPool.Exec(ctx, `UPDATE conflict_example SET value = 'second_tx' WHERE id = 1`)
			if err != nil {
				return err
			}
		}

		// modify the row in the transaction: serialization failure
		_, err = tx.Exec(ctx, `UPDATE conflict_example SET value = 'first_tx' WHERE id = 1`)
		return err
	}
	// must use RepeatableRead or higher isolation to cause the serialization failure
	err = Run(ctx, pgPool, firstTxnBody, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		t.Fatal(err)
	}

	if count != 2 {
		t.Errorf("expected transaction to be retried once; count=%d", count)
	}
}
