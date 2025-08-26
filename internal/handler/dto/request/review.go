package request

import (
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
