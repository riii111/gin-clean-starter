package reservation

import (
	"errors"
	"time"

	"gin-clean-starter/internal/pkg/clock"

	"github.com/google/uuid"
)

var (
	ErrInvalidTimeSlot     = errors.New("invalid time slot")
	ErrLeadTimeNotMet      = errors.New("lead time requirement not met")
	ErrNegativePrice       = errors.New("price cannot be negative")
	ErrReservationCanceled = errors.New("reservation is already canceled")
	ErrInvalidStatus       = errors.New("invalid reservation status")
	ErrInvalidCoupon       = errors.New("invalid coupon")
)

type ResourceSpec struct {
	ID          uuid.UUID
	LeadTimeMin int
}

type CouponSpec struct {
	ID             uuid.UUID
	AmountOffCents *int32
	PercentOff     *float64
	ValidFrom      *time.Time
	ValidTo        *time.Time
}

type ResourcePriceContext struct {
	ResourceID uuid.UUID
}

type Services struct {
	Clock           clock.Clock
	PriceCalculator PriceCalculator
}

type PriceCalculator interface {
	CalculatePriceCents(ctx ResourcePriceContext, slot TimeSlot) int64
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
	res ResourceSpec,
	userID uuid.UUID,
	slot TimeSlot,
	coup *CouponSpec,
	note Note,
) (*Reservation, error) {
	lead := res.LeadTimeMin
	if lead < 0 {
		lead = 0
	}
	if err := slot.ValidateLeadTimeAt(services.Clock.Now(), lead); err != nil {
		return nil, err
	}

	base := services.PriceCalculator.CalculatePriceCents(ResourcePriceContext{ResourceID: res.ID}, slot)
	if base < 0 {
		return nil, ErrNegativePrice
	}

	if coup != nil {
		now := services.Clock.Now()
		if (coup.ValidFrom != nil && now.Before(*coup.ValidFrom)) ||
			(coup.ValidTo != nil && now.After(*coup.ValidTo)) {
			return nil, ErrInvalidCoupon
		}
		base = applyDiscount(base, coup.AmountOffCents, coup.PercentOff)
	}

	price := NewMoney(base)
	var couponID *uuid.UUID
	if coup != nil {
		id := coup.ID
		couponID = &id
	}

	return &Reservation{
		id:         uuid.New(),
		resourceID: res.ID,
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

func (pc *DefaultPriceCalculator) CalculatePriceCents(_ ResourcePriceContext, slot TimeSlot) int64 {
	duration := slot.Duration()
	hours := duration.Hours()
	return int64(hours * float64(pc.HourlyRateCents))
}

func applyDiscount(base int64, amountOff *int32, percentOff *float64) int64 {
	result := base
	if amountOff != nil {
		result -= int64(*amountOff)
	}
	if percentOff != nil {
		result = int64(float64(result) * (100.0 - *percentOff) / 100.0)
	}
	if result < 0 {
		result = 0
	}
	return result
}
