package response

import (
	"time"

	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
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

func FromReservationRM(rm *readmodel.ReservationRM) *ReservationResponse {
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

func FromReservationListRM(rm *readmodel.ReservationListRM) *ReservationListResponse {
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
