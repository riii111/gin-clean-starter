package readmodel

import (
	"time"

	"github.com/google/uuid"
)

type ReservationRM struct {
	ID           uuid.UUID  `json:"id"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	ResourceName string     `json:"resource_name"`
	UserID       uuid.UUID  `json:"user_id"`
	UserEmail    string     `json:"user_email"`
	Slot         string     `json:"slot"`
	Status       string     `json:"status"`
	PriceCents   int32      `json:"price_cents"`
	CouponID     *uuid.UUID `json:"coupon_id,omitempty"`
	CouponCode   *string    `json:"coupon_code,omitempty"`
	Note         *string    `json:"note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ReservationListRM struct {
	ID           uuid.UUID `json:"id"`
	ResourceID   uuid.UUID `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Slot         string    `json:"slot"`
	Status       string    `json:"status"`
	PriceCents   int32     `json:"price_cents"`
	CreatedAt    time.Time `json:"created_at"`
}
