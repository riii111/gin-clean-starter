package reservation

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidTimeSlot     = errors.New("invalid time slot")
	ErrLeadTimeNotMet      = errors.New("lead time requirement not met")
	ErrNegativePrice       = errors.New("price cannot be negative")
	ErrReservationCanceled = errors.New("reservation is already canceled")
	ErrInvalidStatus       = errors.New("invalid reservation status")
)

type Reservation struct {
	id         uuid.UUID
	resourceID uuid.UUID
	userID     uuid.UUID
	timeSlot   TimeSlot
	status     Status
	price      Money
	couponID   *uuid.UUID
	note       Note
	createdAt  time.Time
	updatedAt  time.Time
}

func NewReservation(
	resourceID, userID uuid.UUID,
	timeSlot TimeSlot,
	price Money,
	couponID *uuid.UUID,
	note Note,
	_ int, // leadTimeMinutes - validation moved to Factory
) (*Reservation, error) {
	return &Reservation{
		id:         uuid.New(),
		resourceID: resourceID,
		userID:     userID,
		timeSlot:   timeSlot,
		status:     StatusConfirmed,
		price:      price,
		couponID:   couponID,
		note:       note,
	}, nil
}

func (r *Reservation) IsActive() bool {
	return r.status == StatusConfirmed
}

func (r *Reservation) IsCanceled() bool {
	return r.status == StatusCanceled
}

func (r *Reservation) HasExpired() bool {
	return time.Now().After(r.timeSlot.End())
}

func (r *Reservation) ID() uuid.UUID         { return r.id }
func (r *Reservation) ResourceID() uuid.UUID { return r.resourceID }
func (r *Reservation) UserID() uuid.UUID     { return r.userID }
func (r *Reservation) TimeSlot() TimeSlot    { return r.timeSlot }
func (r *Reservation) Status() Status        { return r.status }
func (r *Reservation) Price() Money          { return r.price }
func (r *Reservation) CouponID() *uuid.UUID  { return r.couponID }
func (r *Reservation) Note() Note            { return r.note }
func (r *Reservation) CreatedAt() time.Time  { return r.createdAt }
func (r *Reservation) UpdatedAt() time.Time  { return r.updatedAt }
