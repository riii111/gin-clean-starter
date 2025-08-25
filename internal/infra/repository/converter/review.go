package converter

import (
	"gin-clean-starter/internal/domain/review"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
)

func ReviewToCreateParams(r *review.Review) sqlc.CreateReviewParams {
	return sqlc.CreateReviewParams{
		UserID:        r.UserID(),
		ResourceID:    r.ResourceID(),
		ReservationID: r.ReservationID(),
		Rating:        int32(r.Rating().Value()),
		Comment:       r.Comment().String(),
	}
}
