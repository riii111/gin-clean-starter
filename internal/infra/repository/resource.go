package repository

import (
	"context"

	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ResourceWriteQueries interface {
	GetAllResources(ctx context.Context, db sqlc.DBTX) ([]sqlc.Resources, error)
	GetResourceByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.Resources, error)
	SearchResourcesByName(ctx context.Context, db sqlc.DBTX, name pgtype.Text) ([]sqlc.Resources, error)
}

type ResourceRepository struct {
	queries ResourceWriteQueries
	db      sqlc.DBTX
}

func NewResourceRepository(queries *sqlc.Queries, db sqlc.DBTX) *ResourceRepository {
	return &ResourceRepository{
		queries: queries,
		db:      db,
	}
}
