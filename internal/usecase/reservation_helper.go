package usecase

import (
	"strings"

	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/domain/resource"

	"github.com/google/uuid"
)

func (r *reservationUseCaseImpl) createReservationEntity(
	params CreateReservationParams,
	resourceEntity *resource.Resource,
	couponEntity *coupon.Coupon,
) (*reservation.Reservation, error) {
	timeSlot, err := reservation.NewTimeSlot(params.StartTime, params.EndTime)
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
	if params.Note != nil {
		trimmedNote := strings.TrimSpace(*params.Note)
		note = reservation.NewNote(trimmedNote)
	}

	var couponID *uuid.UUID
	if couponEntity != nil {
		id := couponEntity.ID()
		couponID = &id
	}

	return reservation.NewReservation(
		params.ResourceID,
		params.UserID,
		timeSlot,
		price,
		couponID,
		note,
		resourceEntity.LeadTimeMin(),
	)
}
