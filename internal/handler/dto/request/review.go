package request

import (
	"time"

	domreview "gin-clean-starter/internal/domain/review"
	"gin-clean-starter/internal/pkg/patch"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
)

type CreateReviewRequest struct {
	ResourceID    uuid.UUID `json:"resourceId" binding:"required"`
	ReservationID uuid.UUID `json:"reservationId" binding:"required"`
	Rating        int       `json:"rating" binding:"required,min=1,max=5"`
	Comment       string    `json:"comment" binding:"required,max=1000"`
}

type UpdateReviewRequest struct {
	Rating  *int    `json:"rating" binding:"omitempty,min=1,max=5"`
	Comment *string `json:"comment" binding:"omitempty,max=1000"`
}

func (r *CreateReviewRequest) ToDomain(userID uuid.UUID, now time.Time) (*domreview.Review, error) {
	return domreview.NewReview(uuid.Nil, userID, r.ResourceID, r.ReservationID, r.Rating, r.Comment, now)
}

func (r *UpdateReviewRequest) ToDomain(existing *shared.ReviewSnapshot, now time.Time) (*domreview.Review, error) {
	rating := patch.Coalesce(r.Rating, existing.Rating)
	comment := patch.Coalesce(r.Comment, existing.Comment)

	return domreview.NewReview(existing.ID, existing.UserID, existing.ResourceID, existing.ReservationID, rating, comment, now)
}
