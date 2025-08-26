package request

import (
	"time"

	domreview "gin-clean-starter/internal/domain/review"
	"gin-clean-starter/internal/pkg/patch"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

type CreateReviewRequest struct {
	ResourceID    uuid.UUID `json:"resource_id" binding:"required"`
	ReservationID uuid.UUID `json:"reservation_id" binding:"required"`
	Rating        int       `json:"rating" binding:"required,min=1,max=5"`
	Comment       string    `json:"comment" binding:"required,max=1000"`
}

type UpdateReviewRequest struct {
	Rating  *int    `json:"rating" binding:"omitempty,min=1,max=5"`
	Comment *string `json:"comment" binding:"omitempty,max=1000"`
}

func (r *CreateReviewRequest) ToDomain() (int, string, error) {
	return r.Rating, r.Comment, nil
}

func (r *UpdateReviewRequest) ToDomain(existing *queries.ReviewView, now time.Time) (*domreview.Review, error) {
	rating := patch.Coalesce(r.Rating, int(existing.Rating))
	comment := patch.Coalesce(r.Comment, existing.Comment)

	return domreview.NewReview(existing.UserID, existing.ResourceID, existing.ReservationID, rating, comment, now)
}
