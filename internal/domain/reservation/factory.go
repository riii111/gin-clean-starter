package reservation

import (
	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/resource"
	"gin-clean-starter/internal/pkg/clock"

	"github.com/google/uuid"
)

type Factory struct {
	Clock           clock.Clock
	PriceCalculator PriceCalculator
}

func NewFactory(clock clock.Clock, priceCalculator PriceCalculator) *Factory {
	return &Factory{
		Clock:           clock,
		PriceCalculator: priceCalculator,
	}
}

func (f *Factory) CreateReservation(
	resourceEntity *resource.Resource,
	userID uuid.UUID,
	slot TimeSlot,
	couponEntity *coupon.Coupon,
	note Note,
) (*Reservation, error) {
	if err := slot.ValidateLeadTimeAt(f.Clock.Now(), resourceEntity.LeadTimeMin()); err != nil {
		return nil, err
	}

	basePriceCents := f.PriceCalculator.CalculatePriceCents(resourceEntity, slot)
	if basePriceCents < 0 {
		return nil, ErrNegativePrice
	}

	if couponEntity != nil {
		if err := couponEntity.ValidateUsage(f.Clock.Now()); err != nil {
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

	return NewReservation(
		resourceEntity.ID(),
		userID,
		slot,
		price,
		couponID,
		note,
		resourceEntity.LeadTimeMin(),
	)
}
