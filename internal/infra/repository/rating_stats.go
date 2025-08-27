package repository

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type RatingStatsQueries interface {
	RecalcResourceRatingStats(ctx context.Context, db sqlc.DBTX, resourceID uuid.UUID) error
}

type RatingStatsRepository struct {
	queries RatingStatsQueries
	db      sqlc.DBTX
}

func NewRatingStatsRepository(queries RatingStatsQueries, db sqlc.DBTX) *RatingStatsRepository {
	return &RatingStatsRepository{
		queries: queries,
		db:      db,
	}
}

func (r *RatingStatsRepository) RecalcResourceRatingStats(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID) error {
	if err := r.queries.RecalcResourceRatingStats(ctx, tx, resourceID); err != nil {
		return infra.WrapRepoErr("failed to recalculate resource rating stats", err)
	}
	return nil
}
