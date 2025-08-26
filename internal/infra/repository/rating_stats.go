package repository

import (
	"context"

	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type RatingStatsRepository struct {
	q  *sqlc.Queries
	db sqlc.DBTX
}

func NewRatingStatsRepository(q *sqlc.Queries, db sqlc.DBTX) *RatingStatsRepository {
	return &RatingStatsRepository{q: q, db: db}
}

func (r *RatingStatsRepository) RecalcResourceRatingStats(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID) error {
	return r.q.RecalcResourceRatingStats(ctx, tx, resourceID)
}
