package repository

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"

	"github.com/google/uuid"
)

type IdempotencyWriteQueries interface {
	TryInsertIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.TryInsertIdempotencyKeyParams) error
	GetIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.GetIdempotencyKeyParams) (sqlc.IdempotencyKeys, error)
	UpdateIdempotencyKeyCompleted(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateIdempotencyKeyCompletedParams) error
	DeleteExpiredIdempotencyKeys(ctx context.Context, db sqlc.DBTX) (int64, error)
}

type IdempotencyRepository struct {
	queries IdempotencyWriteQueries
	db      sqlc.DBTX
}

func NewIdempotencyRepository(queries *sqlc.Queries, db sqlc.DBTX) *IdempotencyRepository {
	return &IdempotencyRepository{
		queries: queries,
		db:      db,
	}
}

func (r *IdempotencyRepository) TryInsert(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error {
	params := sqlc.TryInsertIdempotencyKeyParams{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		ExpiresAt:   pgconv.TimeToPgtype(expiresAt),
	}

	err := r.queries.TryInsertIdempotencyKey(ctx, tx, params)
	if err != nil {
		return infra.WrapRepoErr("failed to try insert idempotency key", err)
	}

	return nil
}

func (r *IdempotencyRepository) UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, responseBodyHash string, resultReservationID uuid.UUID) error {
	params := sqlc.UpdateIdempotencyKeyCompletedParams{
		Key:                 key,
		UserID:              userID,
		ResponseBodyHash:    pgconv.StringToPgtype(responseBodyHash),
		ResultReservationID: pgconv.UUIDToPgtype(resultReservationID),
	}

	err := r.queries.UpdateIdempotencyKeyCompleted(ctx, tx, params)
	if err != nil {
		return infra.WrapRepoErr("failed to update idempotency key status", err)
	}

	return nil
}

func (r *IdempotencyRepository) DeleteExpired(ctx context.Context) (int64, error) {
	count, err := r.queries.DeleteExpiredIdempotencyKeys(ctx, r.db)
	if err != nil {
		return 0, infra.WrapRepoErr("failed to delete expired idempotency keys", err)
	}

	return count, nil
}
