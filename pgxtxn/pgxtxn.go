// Package pgxtxn executes transactions safely and with concurrency retries.
package pgxtxn

import (
	"context"
	"errors"

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

// Run executes body in a transaction that will always commit or roll back,
// and with retries in case of deadlocks or serialization errors. If body returns an error, the
// transaction is rolled back. If it returns nil, the transaction is committed. The body function
// should not call Commit, but may call Rollback. The ctx argument is passed to Begin, Commit,
// Rollback and body without modification.
//
// This prevents the following common mistakes:
// - Forgetting to COMMIT or ROLLBACK in all cases, leaving "stuck" transactions
// - Forgetting to retry on serialization errors
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
			// ErrTxClosed happens if the transaction is already committed/rolled back explicitly
			// but log any other errors (they should not happen)
			err2 := tx.Rollback(ctx)
			if err2 != nil && err2 != pgx.ErrTxClosed {
				slog.LogAttrs(ctx, slog.LevelWarn, "pgtxn.Run: unexpected error when rolling back transaction while handling error",
					slog.Int("attempt", i+1),
					slog.String("rollback_error", err2.Error()),
					slog.String("body_error", err.Error()),
				)
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

		err = tx.Commit(ctx)
		if err != nil && errors.Is(err, pgx.ErrTxClosed) {
			// this transaction was committed or rolled back explicitly: not an error
			err = nil
		}
		return err
	}
	panic("BUG: should not be possible")
}
