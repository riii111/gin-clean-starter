package uow

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"log/slog"
	"time"

	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	pgErrCodeSerializationFailure = "40001"
	pgErrCodeDeadlockDetected     = "40P01"
)

var (
	errTransactionBegin   = errs.New("failed to begin transaction")
	errTransactionCommit  = errs.New("failed to commit transaction")
	errMaxRetriesExceeded = errs.New("transaction failed after max retries")
)

type PostgresUoW struct {
	pool *pgxpool.Pool
	q    *sqlc.Queries

	// write repositories provided via DI
	reservationRepo  shared.ReservationRepository
	reviewRepo       shared.ReviewRepository
	ratingStatsRepo  shared.RatingStatsRepository
	idempotencyRepo  shared.IdempotencyRepository
	notificationRepo shared.NotificationRepository
	userRepo         shared.UserRepository
}

func NewPostgresUoW(
	pool *pgxpool.Pool,
	q *sqlc.Queries,
	reservationRepo shared.ReservationRepository,
	reviewRepo shared.ReviewRepository,
	ratingStatsRepo shared.RatingStatsRepository,
	idempotencyRepo shared.IdempotencyRepository,
	notificationRepo shared.NotificationRepository,
	userRepo shared.UserRepository,
) shared.UnitOfWork {
	return &PostgresUoW{
		pool:             pool,
		q:                q,
		reservationRepo:  reservationRepo,
		reviewRepo:       reviewRepo,
		ratingStatsRepo:  ratingStatsRepo,
		idempotencyRepo:  idempotencyRepo,
		notificationRepo: notificationRepo,
		userRepo:         userRepo,
	}
}

// ReadCommitted prevents dirty reads while allowing concurrent writes
func (u *PostgresUoW) Within(ctx context.Context, fn func(ctx context.Context, tx shared.Tx) error) error {
	return u.runInTxWithOptions(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

// Read-only transaction for consistent multi-table snapshots
func (u *PostgresUoW) DB(_ context.Context) sqlc.DBTX { return u.pool }

// Avoids defer accumulation in retry loops to prevent connection leaks
func (u *PostgresUoW) runInTxWithOptions(ctx context.Context, options pgx.TxOptions, fn func(ctx context.Context, tx shared.Tx) error) error {
	const maxRetries = 3
	base := 100 * time.Millisecond

	for attempt := 0; attempt <= maxRetries; attempt++ {
		pgxTx, err := u.pool.BeginTx(ctx, options)
		if err != nil {
			return errs.Mark(err, errTransactionBegin)
		}

		tx := &pgTx{
			dbtx: pgxTx,
			uow:  u,
		}

		err = fn(ctx, tx)
		if err == nil {
			if err = pgxTx.Commit(ctx); err == nil {
				return nil
			}
			err = errs.Mark(err, errTransactionCommit)
		}

		if rollbackErr := pgxTx.Rollback(ctx); rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("rollback failed", "attempt", attempt+1, "error", rollbackErr.Error())
			}
		}

		if !shouldRetry(err, attempt, maxRetries) {
			if attempt == maxRetries {
				slog.Error("transaction failed after max retries",
					"attempts", attempt+1,
					"error", err.Error())
				return errs.Mark(err, errMaxRetriesExceeded)
			}
			return err
		}

		waitTime := calculateBackoff(attempt, base)

		slog.Warn("retrying transaction due to retryable error",
			"attempt", attempt+1,
			"wait_ms", waitTime.Milliseconds(),
			"error", err.Error())

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return errMaxRetriesExceeded
}

func shouldRetry(err error, attempt, maxRetries int) bool {
	return isRetryableError(err) && attempt < maxRetries
}

func calculateBackoff(attempt int, base time.Duration) time.Duration {
	waitTime := time.Duration(1<<attempt) * base
	jitter := cryptoRandInt63n(int64(waitTime / 5))
	return waitTime + time.Duration(jitter)
}

func cryptoRandInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// Fallback to a simple calculation if crypto/rand fails
		return 0
	}
	// Safe conversion: mask high bit to ensure positive int64
	uval := binary.BigEndian.Uint64(buf[:]) & 0x7FFFFFFFFFFFFFFF
	// #nosec G115 -- Intentionally safe conversion after masking
	return int64(uval) % n
}

func isRetryableError(err error) bool {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return false
	}

	switch pgErr.Code {
	case pgErrCodeSerializationFailure, pgErrCodeDeadlockDetected:
		return true
	default:
		return false
	}
}

type pgTx struct {
	dbtx sqlc.DBTX
	uow  *PostgresUoW
}

func (t *pgTx) DB() sqlc.DBTX {
	return t.dbtx
}

// Expose write repositories via Tx to signal writes occur within a transaction.
func (t *pgTx) Reservations() shared.ReservationRepository {
	return t.uow.reservationRepo
}

func (t *pgTx) Reviews() shared.ReviewRepository {
	return t.uow.reviewRepo
}

func (t *pgTx) RatingStats() shared.RatingStatsRepository {
	return t.uow.ratingStatsRepo
}

func (t *pgTx) Idempotency() shared.IdempotencyRepository {
	return t.uow.idempotencyRepo
}

func (t *pgTx) Notifications() shared.NotificationRepository {
	return t.uow.notificationRepo
}

func (t *pgTx) Users() shared.UserRepository {
	return t.uow.userRepo
}
