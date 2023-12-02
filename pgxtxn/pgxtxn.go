// Package pgxtxn executes transactions safely and with concurrency retries.
package pgxtxn

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/exp/slog"
)

const maxRetries = 3

// PGCodeDeadlockDetected is the Postgres error code for a deadlock. See:
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const PGCodeDeadlockDetected = "40P01"

// PGCodeSerializationFailure is the Postgres error code for a serialization failure. See:
// https://www.postgresql.org/docs/current/errcodes-appendix.html
const PGCodeSerializationFailure = "40001"

// TransactionalDB is a database that can run transactions.
type TransactionalDB interface {
	// Begin is starts a pgx transaction. See pgxpool.BeginTx for details.
	BeginTx(ctx context.Context, txOptions pgx.TxOptions) (pgx.Tx, error)
}

// Run executes body in a transaction without the possibility of forgetting to commit or roll back,
// and with retries in case of deadlocks or serialization errors. If body returns an error, the
// transaction is rolled back. If it returns nil, the transaction is committed. The body function
// must not call Commit, but may call Rollback. The ctx argument is passed to Begin, Commit,
// Rollback and body without modification.
//
// TODO: Pass an interface that does not have Commit to body to avoid mistakes?
func Run(
	ctx context.Context, db TransactionalDB, body func(ctx context.Context, tx pgx.Tx) error,
	txOptions pgx.TxOptions,
) error {

	for i := 0; i < maxRetries; i++ {
		tx, err := db.BeginTx(ctx, txOptions)
		if err != nil {
			return err
		}
		err = body(ctx, tx)
		if err != nil {
			err2 := tx.Rollback(ctx)
			if err2 != nil {
				return fmt.Errorf("%w; pgtxn.Run: rollback failed: %w", err, err2)
			}
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) &&
				(pgErr.Code == PGCodeDeadlockDetected || pgErr.Code == PGCodeSerializationFailure) {
				if i == maxRetries-1 {
					return err
				}

				slog.LogAttrs(ctx, slog.LevelInfo, "pgtxn.Run: retrying transaction",
					slog.Int("attempt", i+1), slog.String("pg_error", pgErr.Error()))
				continue
			}
			return err
		}
		return tx.Commit(ctx)
	}
	panic("BUG: should not be possible")
}
