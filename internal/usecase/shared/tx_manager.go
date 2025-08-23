package shared

import (
	"context"
	"errors"
	"log/slog"
	"time"

	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/errs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTransactionBegin    = errs.New("failed to begin transaction")
	ErrTransactionCommit   = errs.New("failed to commit transaction")
	ErrTransactionRollback = errs.New("failed to rollback transaction")
	ErrMaxRetriesExceeded  = errs.New("transaction failed after max retries")
)

func RunInTx[T any](ctx context.Context, db *pgxpool.Pool, fn func(tx sqlc.DBTX) (T, error)) (T, error) {
	var zero T

	tx, err := db.Begin(ctx)
	if err != nil {
		return zero, errs.Mark(err, ErrTransactionBegin)
	}

	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			// Only log rollback errors for uncommitted transactions
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("failed to rollback transaction", "error", rollbackErr)
			}
		}
	}()

	result, err := fn(tx)
	if err != nil {
		return zero, err
	}

	if err = tx.Commit(ctx); err != nil {
		return zero, errs.Mark(err, ErrTransactionCommit)
	}

	return result, nil
}

func RunInTxWithRetry[T any](ctx context.Context, db *pgxpool.Pool, maxRetries int, fn func(tx sqlc.DBTX) (T, error)) (T, error) {
	var zero T

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := RunInTx(ctx, db, fn)
		if err == nil {
			return result, nil
		}

		if !isRetryableError(err) {
			return zero, err
		}

		if attempt == maxRetries {
			slog.Error("transaction failed after max retries",
				"attempts", attempt+1,
				"error", err)
			return zero, errs.Mark(err, ErrMaxRetriesExceeded)
		}

		// Wait before retrying with exponential backoff
		waitTime := time.Duration(attempt+1) * 100 * time.Millisecond
		slog.Warn("retrying transaction due to retryable error",
			"attempt", attempt+1,
			"wait_time", waitTime,
			"error", err)

		select {
		case <-ctx.Done():
			return zero, ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return zero, ErrMaxRetriesExceeded
}

func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	// PostgreSQL error codes for retryable conditions:
	// 40001: serialization_failure
	// 40P01: deadlock_detected
	switch pgErr.Code {
	case "40001", "40P01":
		return true
	default:
		return false
	}
}

type TransactionFunc[T any] func(tx sqlc.DBTX) (T, error)

func WithDefaultRetry[T any](ctx context.Context, db *pgxpool.Pool, fn TransactionFunc[T]) (T, error) {
	return RunInTxWithRetry(ctx, db, 3, fn)
}
