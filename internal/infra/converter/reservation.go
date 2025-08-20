package converter

import (
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/infra/sqlc"

	"github.com/jackc/pgx/v5/pgtype"
)

func ReservationToInfra(res *reservation.Reservation) sqlc.CreateReservationParams {
	params := sqlc.CreateReservationParams{
		ResourceID: res.ResourceID(),
		UserID:     res.UserID(),
		Slot:       res.TimeSlot().ToTstzrange(),
		Status:     res.Status().String(),
		PriceCents: int32(res.Price().Cents()),
	}

	if couponID := res.CouponID(); couponID != nil {
		params.CouponID = pgtype.UUID{Bytes: *couponID, Valid: true}
	} else {
		params.CouponID = pgtype.UUID{Valid: false}
	}

	noteStr := res.Note().String()
	if noteStr != "" {
		params.Note = pgtype.Text{String: noteStr, Valid: true}
	} else {
		params.Note = pgtype.Text{Valid: false}
	}

	return params
}
