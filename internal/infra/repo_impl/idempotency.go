package repo_impl

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type IdempotencyQueries interface {
	CreateIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateIdempotencyKeyParams) error
	GetIdempotencyKey(ctx context.Context, db sqlc.DBTX, arg sqlc.GetIdempotencyKeyParams) (sqlc.IdempotencyKeys, error)
	UpdateIdempotencyKeyStatus(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateIdempotencyKeyStatusParams) error
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

func (r *IdempotencyRepository) Create(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error {
	params := sqlc.CreateIdempotencyKeyParams{
		Key:         key,
		UserID:      userID,
		Endpoint:    endpoint,
		RequestHash: requestHash,
		Status:      "processing",
		ExpiresAt:   pgtype.Timestamptz{Time: expiresAt, Valid: true},
	}

	err := r.queries.CreateIdempotencyKey(ctx, tx, params)
	if err != nil {
		return infra.WrapRepoErr("failed to create idempotency key", err)
	}

	return nil
}

func (r *IdempotencyRepository) Get(ctx context.Context, key uuid.UUID, userID uuid.UUID) (*readmodel.IdempotencyKeyRM, error) {
	params := sqlc.GetIdempotencyKeyParams{
		Key:    key,
		UserID: userID,
	}

	row, err := r.queries.GetIdempotencyKey(ctx, r.db, params)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("idempotency key not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to get idempotency key", err)
	}

	return toIdempotencyKeyRMFromRow(row), nil
}

func (r *IdempotencyRepository) UpdateStatus(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, status, responseBodyHash string) error {
	params := sqlc.UpdateIdempotencyKeyStatusParams{
		Key:              key,
		UserID:           userID,
		Status:           status,
		ResponseBodyHash: pgtype.Text{String: responseBodyHash, Valid: true},
	}

	err := r.queries.UpdateIdempotencyKeyStatus(ctx, tx, params)
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

func toIdempotencyKeyRMFromRow(row sqlc.IdempotencyKeys) *readmodel.IdempotencyKeyRM {
	rm := &readmodel.IdempotencyKeyRM{
		Key:         row.Key,
		UserID:      row.UserID,
		Endpoint:    row.Endpoint,
		RequestHash: row.RequestHash,
		Status:      row.Status,
		ExpiresAt:   row.ExpiresAt.Time,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}

	if row.ResponseBodyHash.Valid {
		rm.ResponseBodyHash = &row.ResponseBodyHash.String
	}

	return rm
}
