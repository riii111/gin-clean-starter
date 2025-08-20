package readmodel

import (
	"time"

	"github.com/google/uuid"
)

type CouponRM struct {
	ID             uuid.UUID  `json:"id"`
	Code           string     `json:"code"`
	AmountOffCents *int32     `json:"amount_off_cents,omitempty"`
	PercentOff     *float64   `json:"percent_off,omitempty"`
	ValidFrom      *time.Time `json:"valid_from,omitempty"`
	ValidTo        *time.Time `json:"valid_to,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}
