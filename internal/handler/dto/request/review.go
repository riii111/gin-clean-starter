package request

import (
	domreview "gin-clean-starter/internal/domain/review"

	"github.com/google/uuid"
)

type CreateReviewRequest struct {
	ResourceID    uuid.UUID `json:"resource_id" binding:"required"`
	ReservationID uuid.UUID `json:"reservation_id" binding:"required"`
	Rating        int       `json:"rating" binding:"required,min=1,max=5"`
	Comment       string    `json:"comment" binding:"required,max=1000"`
}

type UpdateReviewRequest struct {
	Rating  int    `json:"rating" binding:"required,min=1,max=5"`
	Comment string `json:"comment" binding:"required,max=1000"`
}

// validateRatingAndComment validates rating and comment and returns domain objects
func validateRatingAndComment(rating int, comment string) (domreview.Rating, domreview.Comment, error) {
	domainRating, err := domreview.NewRating(rating)
	if err != nil {
		return domreview.Rating{}, domreview.Comment{}, err
	}

	domainComment, err := domreview.NewComment(comment)
	if err != nil {
		return domreview.Rating{}, domreview.Comment{}, err
	}

	return domainRating, domainComment, nil
}

func (r *CreateReviewRequest) ToDomain() (domreview.Rating, domreview.Comment, error) {
	return validateRatingAndComment(r.Rating, r.Comment)
}

func (r *UpdateReviewRequest) ToDomain() (domreview.Rating, domreview.Comment, error) {
	return validateRatingAndComment(r.Rating, r.Comment)
}
