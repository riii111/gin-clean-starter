package writerepo

import (
	"context"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/pgconv"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ResourceQueries interface {
	GetAllResources(ctx context.Context, db sqlc.DBTX) ([]sqlc.Resources, error)
	GetResourceByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.Resources, error)
	SearchResourcesByName(ctx context.Context, db sqlc.DBTX, name pgtype.Text) ([]sqlc.Resources, error)
}

type ResourceRepository struct {
	queries ResourceQueries
	db      sqlc.DBTX
}

func NewResourceRepository(queries *sqlc.Queries, db sqlc.DBTX) *ResourceRepository {
	return &ResourceRepository{
		queries: queries,
		db:      db,
	}
}

func (r *ResourceRepository) FindAll(ctx context.Context) ([]*queries.ResourceView, error) {
	rows, err := r.queries.GetAllResources(ctx, r.db)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find all resources", err)
	}

	result := make([]*queries.ResourceView, len(rows))
	for i, row := range rows {
		result[i] = toResourceViewFromRow(row)
	}

	return result, nil
}

func (r *ResourceRepository) FindByID(ctx context.Context, id uuid.UUID) (*queries.ResourceView, error) {
	row, err := r.queries.GetResourceByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("resource not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find resource by ID", err)
	}

	return toResourceViewFromRow(row), nil
}

func (r *ResourceRepository) SearchByName(ctx context.Context, name string) ([]*queries.ResourceView, error) {
	nameParam := pgtype.Text{String: name, Valid: true}
	rows, err := r.queries.SearchResourcesByName(ctx, r.db, nameParam)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to search resources by name", err)
	}

	result := make([]*queries.ResourceView, len(rows))
	for i, row := range rows {
		result[i] = toResourceViewFromRow(row)
	}

	return result, nil
}

func toResourceViewFromRow(row sqlc.Resources) *queries.ResourceView {
	return &queries.ResourceView{
		ID:          row.ID,
		Name:        row.Name,
		LeadTimeMin: row.LeadTimeMin,
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
}
