package readstore

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
)

type IdempotencyReadQueries interface {
	GetIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.GetIdempotencyKeyParams) (sqlc.IdempotencyKeys, error)
}

type IdempotencyStore interface {
	Get(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID) (*shared.IdempotencyRecord, error)
}

type IdempotencyReadStore struct {
	queries IdempotencyReadQueries
}

func NewIdempotencyReadStore(queries IdempotencyReadQueries) *IdempotencyReadStore {
	return &IdempotencyReadStore{
		queries: queries,
	}
}

func (r *IdempotencyReadStore) Get(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID) (*shared.IdempotencyRecord, error) {
	params := sqlc.GetIdempotencyKeyParams{
		Key:    key,
		UserID: userID,
	}

	row, err := r.queries.GetIdempotencyKey(ctx, tx, params)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("idempotency key not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to get idempotency key", err)
	}

	record := &shared.IdempotencyRecord{
		Key:                 row.Key,
		UserID:              row.UserID,
		Status:              row.Status,
		RequestHash:         row.RequestHash,
		ResultReservationID: pgconv.UUIDPtrFromPgtype(row.ResultReservationID),
		ExpiresAt:           pgconv.TimeFromPgtype(row.ExpiresAt),
	}

	if time.Now().After(record.ExpiresAt) {
		return nil, infra.WrapRepoErr("idempotency key expired", nil, infra.KindNotFound)
	}

	return record, nil
}
