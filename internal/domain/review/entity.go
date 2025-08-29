package review

import (
	"time"

	"gin-clean-starter/internal/pkg/errs"

	"github.com/google/uuid"
)

var (
	ErrReservationNotEligible = errs.New("reservation is not eligible for review")
	ErrReviewAlreadyExists    = errs.New("review already exists for this reservation")
)

type Review struct {
	id            uuid.UUID
	userID        uuid.UUID
	resourceID    uuid.UUID
	reservationID uuid.UUID
	rating        Rating
	comment       Comment
	createdAt     time.Time
	updatedAt     time.Time
}

func NewReview(id, userID, resourceID, reservationID uuid.UUID, ratingValue int, commentText string, now time.Time) (*Review, error) {
	rating, err := NewRating(ratingValue)
	if err != nil {
		return nil, err
	}

	comment, err := NewComment(commentText)
	if err != nil {
		return nil, err
	}

	if id == uuid.Nil {
		id = uuid.New()
	}

	return &Review{
		id:            id,
		userID:        userID,
		resourceID:    resourceID,
		reservationID: reservationID,
		rating:        rating,
		comment:       comment,
		createdAt:     now,
		updatedAt:     now,
	}, nil
}

func (r *Review) ID() uuid.UUID            { return r.id }
func (r *Review) UserID() uuid.UUID        { return r.userID }
func (r *Review) ResourceID() uuid.UUID    { return r.resourceID }
func (r *Review) ReservationID() uuid.UUID { return r.reservationID }
func (r *Review) Rating() Rating           { return r.rating }
func (r *Review) Comment() Comment         { return r.comment }
func (r *Review) CreatedAt() time.Time     { return r.createdAt }
func (r *Review) UpdatedAt() time.Time     { return r.updatedAt }
