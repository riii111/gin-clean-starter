package uow

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"log/slog"
	"strings"
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

func (u *PostgresUoW) CommandReads() shared.CommandReads {
	return &commandReads{uow: u, dbtx: u.pool}
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

func (u *PostgresUoW) runReadOnlyTx(ctx context.Context, options pgx.TxOptions, fn func(ctx context.Context, db sqlc.DBTX) error) error {
	pgxTx, err := u.pool.BeginTx(ctx, options)
	if err != nil {
		return errs.Mark(err, errTransactionBegin)
	}

	defer func() {
		if rollbackErr := pgxTx.Rollback(ctx); rollbackErr != nil {
			if !errors.Is(rollbackErr, pgx.ErrTxClosed) {
				slog.Warn("failed to rollback read-only transaction", "error", rollbackErr.Error())
			}
		}
	}()

	if err := fn(ctx, pgxTx); err != nil {
		return err
	}

	return pgxTx.Commit(ctx)
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

	// Lazy-initialized repositories
	reservationRepo  shared.ReservationRepository
	reviewRepo       shared.ReviewRepository
	ratingStatsRepo  shared.RatingStatsRepository
	idempotencyRepo  shared.IdempotencyRepository
	notificationRepo shared.NotificationRepository
	userRepo         shared.UserRepository
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

func (t *pgTx) Reviews() shared.ReviewRepository {
	if t.reviewRepo == nil {
		t.reviewRepo = repository.NewReviewRepository(t.uow.q, t.dbtx)
	}
	return t.reviewRepo
}

func (t *pgTx) RatingStats() shared.RatingStatsRepository {
	if t.ratingStatsRepo == nil {
		t.ratingStatsRepo = repository.NewRatingStatsRepository(t.uow.q, t.dbtx)
	}
	return t.ratingStatsRepo
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

func (t *pgTx) Users() shared.UserRepository {
	if t.userRepo == nil {
		t.userRepo = repository.NewUserRepository(t.uow.q)
	}
	return t.userRepo
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
	reviewStore      *readstore.ReviewReadStore
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

	snapshot := &shared.ResourceSnapshot{
		ID:          resource.ID,
		Name:        resource.Name,
		LeadTimeMin: resource.LeadTimeMin,
	}
	return snapshot, nil
}

func (r *commandReads) CouponByCode(ctx context.Context, code string) (*shared.CouponSnapshot, error) {
	if r.couponStore == nil {
		r.couponStore = readstore.NewCouponReadStore(r.uow.q, r.dbtx)
	}

	coupon, err := r.couponStore.FindByCode(ctx, code)
	if err != nil {
		return nil, err
	}

	snapshot := &shared.CouponSnapshot{
		ID:             coupon.ID,
		Code:           coupon.Code,
		AmountOffCents: coupon.AmountOffCents,
		PercentOff:     coupon.PercentOff,
		ValidFrom:      coupon.ValidFrom,
		ValidTo:        coupon.ValidTo,
	}
	return snapshot, nil
}

func (r *commandReads) ReservationByID(ctx context.Context, id uuid.UUID) (*shared.ReservationSnapshot, error) {
	if r.reservationStore == nil {
		r.reservationStore = readstore.NewReservationReadStore(r.uow.q, r.dbtx)
	}

	reservation, err := r.reservationStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	endTime := parseSlotEndTime(reservation.Slot)
	snapshot := &shared.ReservationSnapshot{
		ID:         reservation.ID,
		ResourceID: reservation.ResourceID,
		UserID:     reservation.UserID,
		Status:     reservation.Status,
		EndTime:    endTime,
	}
	return snapshot, nil
}

func parseSlotEndTime(slot string) time.Time {
	parts := strings.Split(slot, "/")
	if len(parts) != 2 {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, parts[1])
	if err != nil {
		return time.Time{}
	}
	return t
}

func (r *commandReads) IdempotencyByKey(ctx context.Context, key, userID uuid.UUID) (*shared.IdempotencyRecord, error) {
	if r.idempotencyStore == nil {
		r.idempotencyStore = readstore.NewIdempotencyReadStore(r.uow.q)
	}

	record, err := r.idempotencyStore.Get(ctx, r.dbtx, key, userID)
	if err != nil {
		return nil, err
	}

	snapshot := &shared.IdempotencyRecord{
		Key:                 record.Key,
		UserID:              record.UserID,
		Status:              record.Status,
		RequestHash:         record.RequestHash,
		ResultReservationID: record.ResultReservationID,
		ExpiresAt:           record.ExpiresAt,
	}
	return snapshot, nil
}

func (r *commandReads) ReviewByID(ctx context.Context, id uuid.UUID) (*shared.ReviewSnapshot, error) {
	if r.reviewStore == nil {
		r.reviewStore = readstore.NewReviewReadStore(r.uow.q, r.dbtx)
	}
	rv, err := r.reviewStore.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	snap := &shared.ReviewSnapshot{
		ID:            rv.ID,
		UserID:        rv.UserID,
		ResourceID:    rv.ResourceID,
		ReservationID: rv.ReservationID,
		Rating:        int(rv.Rating),
		Comment:       rv.Comment,
	}
	return snap, nil
}
