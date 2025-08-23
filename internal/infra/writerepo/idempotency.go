package writerepo

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/pgconv"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

type IdempotencyQueries interface {
	TryInsertIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.TryInsertIdempotencyKeyParams) error
	GetIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.GetIdempotencyKeyParams) (sqlc.IdempotencyKeys, error)
	UpdateIdempotencyKeyCompleted(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateIdempotencyKeyCompletedParams) error
	DeleteExpiredIdempotencyKeys(ctx context.Context, db sqlc.DBTX) (int64, error)
}

type IdempotencyRepository struct {
	queries IdempotencyQueries
	db      sqlc.DBTX
}

func NewIdempotencyRepository(queries *sqlc.Queries, db sqlc.DBTX) *IdempotencyRepository {
	return &IdempotencyRepository{
		queries: queries,
		db:      db,
	}
}

func (r *IdempotencyRepository) TryInsert(ctx context.Context, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error {
	params := sqlc.TryInsertIdempotencyKeyParams{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		ExpiresAt:   pgconv.TimeToPgtype(expiresAt),
	}

	err := r.queries.TryInsertIdempotencyKey(ctx, r.db, params)
	if err != nil {
		return infra.WrapRepoErr("failed to try insert idempotency key", err)
	}

	return nil
}

func (r *IdempotencyRepository) Get(ctx context.Context, key uuid.UUID, userID uuid.UUID) (*queries.IdempotencyKeyView, error) {
	params := sqlc.GetIdempotencyKeyParams{
		Key:    key,
		UserID: userID,
	}

	row, err := r.queries.GetIdempotencyKey(ctx, r.db, params)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("idempotency key not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to get idempotency key", err)
	}

	rm := toIdempotencyKeyViewFromRow(row)

	// Check if key has expired (treat as not found)
	if time.Now().After(rm.ExpiresAt) {
		return nil, infra.WrapRepoErr("idempotency key expired", nil, infra.KindNotFound)
	}

	return rm, nil
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

func toIdempotencyKeyViewFromRow(row sqlc.IdempotencyKeys) *queries.IdempotencyKeyView {
	rm := &queries.IdempotencyKeyView{
		Key:                 row.Key,
		UserID:              row.UserID,
		Endpoint:            row.Endpoint,
		RequestHash:         row.RequestHash,
		Status:              row.Status,
		ResponseBodyHash:    pgconv.StringPtrFromPgtype(row.ResponseBodyHash),
		ResultReservationID: pgconv.UUIDPtrFromPgtype(row.ResultReservationID),
		ExpiresAt:           pgconv.TimeFromPgtype(row.ExpiresAt),
		CreatedAt:           pgconv.TimeFromPgtype(row.CreatedAt),
		UpdatedAt:           pgconv.TimeFromPgtype(row.UpdatedAt),
	}

	return rm
}
