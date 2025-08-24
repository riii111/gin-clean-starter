package uow

import (
	"context"
	"errors"
	"log/slog"
	"math/rand"
	"time"

	"gin-clean-starter/internal/infra/readstore"
	"gin-clean-starter/internal/infra/repository"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jinzhu/copier"
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
}

func NewPostgresUoW(pool *pgxpool.Pool, q *sqlc.Queries) shared.UnitOfWork {
	return &PostgresUoW{
		pool: pool,
		q:    q,
	}
}

// ReadCommitted prevents dirty reads while allowing concurrent writes
func (u *PostgresUoW) Within(ctx context.Context, fn func(ctx context.Context, tx shared.Tx) error) error {
	return u.runInTxWithOptions(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted}, fn)
}

// Read-only transaction for consistent multi-table snapshots
func (u *PostgresUoW) WithinReadOnly(ctx context.Context, fn func(ctx context.Context, db sqlc.DBTX) error) error {
	return u.runReadOnlyTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly}, fn)
}

func (u *PostgresUoW) WithDB(ctx context.Context, fn func(ctx context.Context, db sqlc.DBTX) error) error {
	return fn(ctx, u.pool)
}

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
				slog.Warn("rollback failed", "attempt", attempt+1, "error", rollbackErr)
			}
		}

		if !isRetryableError(err) || attempt == maxRetries {
			if attempt == maxRetries {
				slog.Error("transaction failed after max retries",
					"attempts", attempt+1,
					"error", err)
				return errs.Mark(err, errMaxRetriesExceeded)
			}
			return err
		}

		// Jitter prevents thundering herd
		waitTime := time.Duration(1<<attempt) * base
		jitter := time.Duration(rand.Int63n(int64(waitTime / 5)))
		waitTime += jitter

		slog.Warn("retrying transaction due to retryable error",
			"attempt", attempt+1,
			"wait_ms", waitTime.Milliseconds(),
			"error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}

	return errMaxRetriesExceeded
}

func (u *PostgresUoW) runReadOnlyTx(ctx context.Context, options pgx.TxOptions, fn func(ctx context.Context, db sqlc.DBTX) error) error {
	pgxTx, err := u.pool.BeginTx(ctx, options)
	if err != nil {
		return errs.Mark(err, errTransactionBegin)
	}

	defer func() {
		if rollbackErr := pgxTx.Rollback(ctx); rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("failed to rollback read-only transaction", "error", rollbackErr)
			}
		}
	}()

	if err := fn(ctx, pgxTx); err != nil {
		return err
	}

	return pgxTx.Commit(ctx)
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

	// Lazy-initialized repositories
	reservationRepo  shared.ReservationRepository
	idempotencyRepo  shared.IdempotencyRepository
	notificationRepo shared.NotificationRepository
	commandReads     shared.CommandReads
}

func (t *pgTx) DB() sqlc.DBTX {
	return t.dbtx
}

func (t *pgTx) Reservations() shared.ReservationRepository {
	if t.reservationRepo == nil {
		t.reservationRepo = repository.NewReservationRepository(t.uow.q, t.dbtx)
	}
	return t.reservationRepo
}

func (t *pgTx) Idempotency() shared.IdempotencyRepository {
	if t.idempotencyRepo == nil {
		t.idempotencyRepo = repository.NewIdempotencyRepository(t.uow.q, t.dbtx)
	}
	return t.idempotencyRepo
}

func (t *pgTx) Notifications() shared.NotificationRepository {
	if t.notificationRepo == nil {
		t.notificationRepo = repository.NewNotificationRepository(t.uow.q, t.dbtx)
	}
	return t.notificationRepo
}

func (t *pgTx) Reads() shared.CommandReads {
	if t.commandReads == nil {
		t.commandReads = &commandReads{
			uow:  t.uow,
			dbtx: t.dbtx,
		}
	}
	return t.commandReads
}

type commandReads struct {
	uow  *PostgresUoW
	dbtx sqlc.DBTX

	// Lazy-initialized readstores
	resourceStore    *readstore.ResourceReadStore
	couponStore      *readstore.CouponReadStore
	reservationStore *readstore.ReservationReadStore
	idempotencyStore *readstore.IdempotencyReadStore
}

func (r *commandReads) ResourceByID(ctx context.Context, id uuid.UUID) (*shared.ResourceSnapshot, error) {
	if r.resourceStore == nil {
		r.resourceStore = readstore.NewResourceReadStore(r.uow.q, r.dbtx)
	}

	resource, err := r.resourceStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var snapshot shared.ResourceSnapshot
	if err := copier.Copy(&snapshot, resource); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *commandReads) CouponByCode(ctx context.Context, code string) (*shared.CouponSnapshot, error) {
	if r.couponStore == nil {
		r.couponStore = readstore.NewCouponReadStore(r.uow.q, r.dbtx)
	}

	coupon, err := r.couponStore.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	var snapshot shared.CouponSnapshot
	if err := copier.Copy(&snapshot, coupon); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *commandReads) ReservationByID(ctx context.Context, id uuid.UUID) (*shared.ReservationSnapshot, error) {
	if r.reservationStore == nil {
		r.reservationStore = readstore.NewReservationReadStore(r.uow.q, r.dbtx)
	}

	reservation, err := r.reservationStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	var snapshot shared.ReservationSnapshot
	if err := copier.Copy(&snapshot, reservation); err != nil {
		return nil, err
	}
	return &snapshot, nil
}

func (r *commandReads) IdempotencyByKey(ctx context.Context, key, userID uuid.UUID) (*shared.IdempotencyRecord, error) {
	if r.idempotencyStore == nil {
		r.idempotencyStore = readstore.NewIdempotencyReadStore(r.uow.q)
	}

	record, err := r.idempotencyStore.Get(ctx, r.dbtx, key, userID)
	if err != nil {
		return nil, err
	}

	var snapshot shared.IdempotencyRecord
	if err := copier.Copy(&snapshot, record); err != nil {
		return nil, err
	}
	return &snapshot, nil
}
