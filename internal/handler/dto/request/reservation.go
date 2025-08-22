package request

import (
	"strings"
	"time"

	"gin-clean-starter/internal/domain/reservation"

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

type DomainConversion struct {
	TimeSlot reservation.TimeSlot
	Note     reservation.Note
}

func (r CreateReservationRequest) ToDomain() (*DomainConversion, error) {
	timeSlot, err := reservation.NewTimeSlot(r.StartTime, r.EndTime)
	if err != nil {
		return nil, err
	}

	note, err := reservation.NewNote("")
	if err != nil {
		return nil, err
	}

	if r.Note != nil {
		note, err = reservation.NewNote(*r.Note)
		if err != nil {
			return nil, err
		}
	}

	return &DomainConversion{
		TimeSlot: timeSlot,
		Note:     note,
	}, nil
}
