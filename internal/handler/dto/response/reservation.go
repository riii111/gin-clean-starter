package response

import (
	"time"

	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
)

type ReservationResponse struct {
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

type ReservationListResponse struct {
	ID           uuid.UUID `json:"id"`
	ResourceID   uuid.UUID `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Slot         string    `json:"slot"`
	Status       string    `json:"status"`
	PriceCents   int32     `json:"price_cents"`
	CreatedAt    time.Time `json:"created_at"`
}

func FromReservationView(rm *queries.ReservationView) *ReservationResponse {
	var response ReservationResponse
	if err := copier.Copy(&response, rm); err != nil {
		panic("failed to copy ReservationView: " + err.Error())
	}
	return &response
}

func FromReservationListItem(rm *queries.ReservationListItem) *ReservationListResponse {
	var response ReservationListResponse
	if err := copier.Copy(&response, rm); err != nil {
		panic("failed to copy ReservationListItem: " + err.Error())
	}
	return &response
}
