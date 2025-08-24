package response

import (
	"time"

	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
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
	return &ReservationResponse{
		ID:           rm.ID,
		ResourceID:   rm.ResourceID,
		ResourceName: rm.ResourceName,
		UserID:       rm.UserID,
		UserEmail:    rm.UserEmail,
		Slot:         rm.Slot,
		Status:       rm.Status,
		PriceCents:   rm.PriceCents,
		CouponID:     rm.CouponID,
		CouponCode:   rm.CouponCode,
		Note:         rm.Note,
		CreatedAt:    rm.CreatedAt,
		UpdatedAt:    rm.UpdatedAt,
	}
}

func FromReservationListItem(rm *queries.ReservationListItem) *ReservationListResponse {
	return &ReservationListResponse{
		ID:           rm.ID,
		ResourceID:   rm.ResourceID,
		ResourceName: rm.ResourceName,
		Slot:         rm.Slot,
		Status:       rm.Status,
		PriceCents:   rm.PriceCents,
		CreatedAt:    rm.CreatedAt,
	}
}
