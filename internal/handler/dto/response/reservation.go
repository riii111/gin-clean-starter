package response

import (
	"time"

	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
)

type ReservationResponse struct {
	ID           uuid.UUID  `json:"id"`
	ResourceID   uuid.UUID  `json:"resourceId"`
	ResourceName string     `json:"resourceName"`
	UserID       uuid.UUID  `json:"userId"`
	UserEmail    string     `json:"userEmail"`
	Slot         string     `json:"slot"`
	Status       string     `json:"status"`
	PriceCents   int32      `json:"priceCents"`
	CouponID     *uuid.UUID `json:"couponId,omitempty"`
	CouponCode   *string    `json:"couponCode,omitempty"`
	Note         *string    `json:"note,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type ReservationListResponse struct {
	ID           uuid.UUID `json:"id"`
	ResourceID   uuid.UUID `json:"resourceId"`
	ResourceName string    `json:"resourceName"`
	Slot         string    `json:"slot"`
	Status       string    `json:"status"`
	PriceCents   int32     `json:"priceCents"`
	CreatedAt    time.Time `json:"createdAt"`
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
