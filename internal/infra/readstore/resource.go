package readstore

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ResourceReadQueries interface {
	GetAllResources(ctx context.Context, db sqlc.DBTX) ([]sqlc.Resources, error)
	GetResourceByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.Resources, error)
	SearchResourcesByName(ctx context.Context, db sqlc.DBTX, name pgtype.Text) ([]sqlc.Resources, error)
}

type ResourceStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*shared.ResourceSnapshot, error)
	FindAll(ctx context.Context) ([]*shared.ResourceSnapshot, error)
	SearchByName(ctx context.Context, name string) ([]*shared.ResourceSnapshot, error)
}

type ResourceReadStore struct {
	queries ResourceReadQueries
	db      sqlc.DBTX
}

func NewResourceReadStore(queries *sqlc.Queries, db sqlc.DBTX) *ResourceReadStore {
	return &ResourceReadStore{
		queries: queries,
		db:      db,
	}
}

func (r *ResourceReadStore) FindAll(ctx context.Context) ([]*shared.ResourceSnapshot, error) {
	rows, err := r.queries.GetAllResources(ctx, r.db)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find all resources", err)
	}

	result := make([]*shared.ResourceSnapshot, len(rows))
	for i, row := range rows {
		result[i] = toResourceSnapshotFromRow(row)
	}

	return result, nil
}

func (r *ResourceReadStore) FindByID(ctx context.Context, id uuid.UUID) (*shared.ResourceSnapshot, error) {
	row, err := r.queries.GetResourceByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("resource not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find resource by ID", err)
	}

	return toResourceSnapshotFromRow(row), nil
}

func (r *ResourceReadStore) SearchByName(ctx context.Context, name string) ([]*shared.ResourceSnapshot, error) {
	nameParam := pgtype.Text{String: name, Valid: true}
	rows, err := r.queries.SearchResourcesByName(ctx, r.db, nameParam)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to search resources by name", err)
	}

	result := make([]*shared.ResourceSnapshot, len(rows))
	for i, row := range rows {
		result[i] = toResourceSnapshotFromRow(row)
	}

	return result, nil
}

func toResourceSnapshotFromRow(row sqlc.Resources) *shared.ResourceSnapshot {
	return &shared.ResourceSnapshot{
		ID:          row.ID,
		Name:        row.Name,
		LeadTimeMin: int(row.LeadTimeMin),
	}
}
