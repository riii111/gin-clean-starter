package reservation

import (
	"gin-clean-starter/internal/domain/resource"
)

type PriceCalculator interface {
	CalculatePriceCents(res *resource.Resource, slot TimeSlot) int64
}

type DefaultPriceCalculator struct {
	HourlyRateCents int64
}

func NewDefaultPriceCalculator() *DefaultPriceCalculator {
	return &DefaultPriceCalculator{
		HourlyRateCents: 100000, // 1000円/時間 (1000 * 100 cents)
	}
}

func (pc *DefaultPriceCalculator) CalculatePriceCents(_ *resource.Resource, slot TimeSlot) int64 {
	duration := slot.Duration()
	hours := duration.Hours()
	return int64(hours * float64(pc.HourlyRateCents))
}
