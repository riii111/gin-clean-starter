package repository

import (
	"context"

	"gin-clean-starter/internal/domain/review"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/repository/converter"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type ReviewWriteQueries interface {
	CreateReview(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateReviewParams) (sqlc.Reviews, error)
	UpdateReview(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateReviewParams) error
	DeleteReview(ctx context.Context, db sqlc.DBTX, id uuid.UUID) error
}

type ReviewRepository struct {
	queries ReviewWriteQueries
	db      sqlc.DBTX
}

func NewReviewRepository(queries *sqlc.Queries, db sqlc.DBTX) *ReviewRepository {
	return &ReviewRepository{
		queries: queries,
		db:      db,
	}
}

func (r *ReviewRepository) Create(ctx context.Context, tx sqlc.DBTX, rev *review.Review) (uuid.UUID, error) {
	params := converter.ReviewToCreateParams(rev)
	row, err := r.queries.CreateReview(ctx, tx, params)
	if err != nil {
		return uuid.Nil, infra.WrapRepoErr("failed to create review", err)
	}
	return row.ID, nil
}

func (r *ReviewRepository) Update(ctx context.Context, tx sqlc.DBTX, rev *review.Review) error {
	params := sqlc.UpdateReviewParams{
		ID:      rev.ID(),
		Rating:  int32(rev.Rating().Value()),
		Comment: rev.Comment().String(),
	}
	if err := r.queries.UpdateReview(ctx, tx, params); err != nil {
		return infra.WrapRepoErr("failed to update review", err)
	}
	return nil
}

func (r *ReviewRepository) Delete(ctx context.Context, tx sqlc.DBTX, reviewID uuid.UUID) error {
	if err := r.queries.DeleteReview(ctx, tx, reviewID); err != nil {
		return infra.WrapRepoErr("failed to delete review", err)
	}
	return nil
}
