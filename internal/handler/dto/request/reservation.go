package request

import (
	"strings"
	"time"

	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/domain/resource"

	"github.com/google/uuid"
)

type CreateReservationRequest struct {
	ResourceID uuid.UUID `json:"resource_id" binding:"required"`
	StartTime  time.Time `json:"start_time" binding:"required"`
	EndTime    time.Time `json:"end_time" binding:"required"`
	CouponCode *string   `json:"coupon_code,omitempty"`
	Note       *string   `json:"note,omitempty"`
}

func (r CreateReservationRequest) GetCouponCode() *string {
	if r.CouponCode == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*r.CouponCode)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (r CreateReservationRequest) ToDomain(
	userID uuid.UUID,
	resourceEntity *resource.Resource,
	couponEntity *coupon.Coupon,
) (*reservation.Reservation, error) {
	timeSlot, err := reservation.NewTimeSlot(r.StartTime, r.EndTime)
	if err != nil {
		return nil, err
	}

	if err := timeSlot.ValidateLeadTime(resourceEntity.LeadTimeMin()); err != nil {
		return nil, err
	}

	// Calculate price in usecase layer
	duration := timeSlot.Duration()
	hours := duration.Hours()
	basePriceCents := int64(hours * 1000 * 100) // 1000円/時間

	// Apply coupon discount if available
	if couponEntity != nil {
		basePriceCents = couponEntity.ApplyDiscount(basePriceCents)
	}

	price := reservation.NewMoney(basePriceCents)

	note := reservation.NewNote("")
	if r.Note != nil {
		trimmedNote := strings.TrimSpace(*r.Note)
		note = reservation.NewNote(trimmedNote)
	}

	var couponID *uuid.UUID
	if couponEntity != nil {
		id := couponEntity.ID()
		couponID = &id
	}

	return reservation.NewReservation(
		r.ResourceID,
		userID,
		timeSlot,
		price,
		couponID,
		note,
		resourceEntity.LeadTimeMin(),
	)
}
