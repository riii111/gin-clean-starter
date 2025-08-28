package repository

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type RatingStatsQueries interface {
	ApplyResourceRatingStatsOnCreate(ctx context.Context, db sqlc.DBTX, arg sqlc.ApplyResourceRatingStatsOnCreateParams) error
	ApplyResourceRatingStatsOnUpdate(ctx context.Context, db sqlc.DBTX, arg sqlc.ApplyResourceRatingStatsOnUpdateParams) error
	ApplyResourceRatingStatsOnDelete(ctx context.Context, db sqlc.DBTX, arg sqlc.ApplyResourceRatingStatsOnDeleteParams) error
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

func (r *RatingStatsRepository) ApplyOnCreate(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID, rating int) error {
	if err := r.queries.ApplyResourceRatingStatsOnCreate(ctx, tx, sqlc.ApplyResourceRatingStatsOnCreateParams{
		ResourceID: resourceID,
		Rating:     int32(rating),
	}); err != nil {
		return infra.WrapRepoErr("failed to apply rating stats on create", err)
	}
	return nil
}

func (r *RatingStatsRepository) ApplyOnUpdate(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID, oldRating, newRating int) error {
	if err := r.queries.ApplyResourceRatingStatsOnUpdate(ctx, tx, sqlc.ApplyResourceRatingStatsOnUpdateParams{
		ResourceID: resourceID,
		OldRating:  int32(oldRating),
		NewRating:  int32(newRating),
	}); err != nil {
		return infra.WrapRepoErr("failed to apply rating stats on update", err)
	}
	return nil
}

func (r *RatingStatsRepository) ApplyOnDelete(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID, oldRating int) error {
	if err := r.queries.ApplyResourceRatingStatsOnDelete(ctx, tx, sqlc.ApplyResourceRatingStatsOnDeleteParams{
		ResourceID: resourceID,
		Rating:     int32(oldRating),
	}); err != nil {
		return infra.WrapRepoErr("failed to apply rating stats on delete", err)
	}
	return nil
}
