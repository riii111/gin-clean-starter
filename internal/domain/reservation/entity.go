package reservation

import (
	"errors"
	"time"

	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/resource"
	"gin-clean-starter/internal/pkg/clock"

	"github.com/google/uuid"
)

var (
	ErrInvalidTimeSlot     = errors.New("invalid time slot")
	ErrLeadTimeNotMet      = errors.New("lead time requirement not met")
	ErrNegativePrice       = errors.New("price cannot be negative")
	ErrReservationCanceled = errors.New("reservation is already canceled")
	ErrInvalidStatus       = errors.New("invalid reservation status")
)

type Services struct {
	Clock           clock.Clock
	PriceCalculator PriceCalculator
}

type PriceCalculator interface {
	CalculatePriceCents(res *resource.Resource, slot TimeSlot) int64
}

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
	services *Services,
	resourceEntity *resource.Resource,
	userID uuid.UUID,
	slot TimeSlot,
	couponEntity *coupon.Coupon,
	note Note,
) (*Reservation, error) {
	if err := slot.ValidateLeadTimeAt(services.Clock.Now(), resourceEntity.LeadTimeMin()); err != nil {
		return nil, err
	}

	basePriceCents := services.PriceCalculator.CalculatePriceCents(resourceEntity, slot)
	if basePriceCents < 0 {
		return nil, ErrNegativePrice
	}

	if couponEntity != nil {
		if err := couponEntity.ValidateUsage(services.Clock.Now()); err != nil {
			return nil, err
		}
		basePriceCents = couponEntity.ApplyDiscount(basePriceCents)
	}

	price := NewMoney(basePriceCents)

	var couponID *uuid.UUID
	if couponEntity != nil {
		id := couponEntity.ID()
		couponID = &id
	}

	return &Reservation{
		id:         uuid.New(),
		resourceID: resourceEntity.ID(),
		userID:     userID,
		timeSlot:   slot,
		status:     StatusConfirmed,
		price:      price,
		couponID:   couponID,
		note:       note,
	}, nil
}

func ReconstructReservation(
	id, resourceID, userID uuid.UUID,
	timeSlot TimeSlot,
	status Status,
	price Money,
	couponID *uuid.UUID,
	note Note,
	createdAt, updatedAt time.Time,
) *Reservation {
	return &Reservation{
		id:         id,
		resourceID: resourceID,
		userID:     userID,
		timeSlot:   timeSlot,
		status:     status,
		price:      price,
		couponID:   couponID,
		note:       note,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}
}

func (r *Reservation) IsActive() bool {
	return r.status == StatusConfirmed
}

func (r *Reservation) IsCanceled() bool {
	return r.status == StatusCanceled
}

func (r *Reservation) HasExpired(now time.Time) bool {
	return now.After(r.timeSlot.End())
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

type DefaultPriceCalculator struct {
	HourlyRateCents int64
}

func NewDefaultPriceCalculator() *DefaultPriceCalculator {
	return &DefaultPriceCalculator{
		HourlyRateCents: 100000,
	}
}

func (pc *DefaultPriceCalculator) CalculatePriceCents(_ *resource.Resource, slot TimeSlot) int64 {
	duration := slot.Duration()
	hours := duration.Hours()
	return int64(hours * float64(pc.HourlyRateCents))
}
