package converter

import (
	"gin-clean-starter/internal/domain/review"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"

	"github.com/google/uuid"
)

func ReviewToCreateParams(r *review.Review) sqlc.CreateReviewParams {
	return sqlc.CreateReviewParams{
		UserID:        r.UserID(),
		ResourceID:    r.ResourceID(),
		ReservationID: r.ReservationID(),
		Rating:        pgconv.IntToInt32(r.Rating().Value()),
		Comment:       r.Comment().String(),
	}
}

func ReviewToUpdateParams(id uuid.UUID, r *review.Review) sqlc.UpdateReviewParams {
	return sqlc.UpdateReviewParams{
		ID:      id,
		Rating:  pgconv.IntToInt32(r.Rating().Value()),
		Comment: r.Comment().String(),
	}
}
