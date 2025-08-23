package repository

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/commands"

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

func (r *ResourceRepository) FindAll(ctx context.Context) ([]*commands.ResourceSnapshot, error) {
	rows, err := r.queries.GetAllResources(ctx, r.db)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find all resources", err)
	}

	result := make([]*commands.ResourceSnapshot, len(rows))
	for i, row := range rows {
		result[i] = toResourceSnapshotFromRow(row)
	}

	return result, nil
}

func (r *ResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*commands.ResourceSnapshot, error) {
	row, err := r.queries.GetResourceByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("resource not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find resource by ID", err)
	}

	return toResourceSnapshotFromRow(row), nil
}

func (r *ResourceRepository) SearchByName(ctx context.Context, name string) ([]*commands.ResourceSnapshot, error) {
	nameParam := pgtype.Text{String: name, Valid: true}
	rows, err := r.queries.SearchResourcesByName(ctx, r.db, nameParam)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to search resources by name", err)
	}

	result := make([]*commands.ResourceSnapshot, len(rows))
	for i, row := range rows {
		result[i] = toResourceSnapshotFromRow(row)
	}

	return result, nil
}

func toResourceSnapshotFromRow(row sqlc.Resources) *commands.ResourceSnapshot {
	return &commands.ResourceSnapshot{
		ID:          row.ID,
		Name:        row.Name,
		LeadTimeMin: int(row.LeadTimeMin),
	}
}
