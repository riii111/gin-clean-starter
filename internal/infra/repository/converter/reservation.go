package converter

import (
	"fmt"
	"math"
	"time"

	"gin-clean-starter/internal/domain/reservation"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/jackc/pgx/v5/pgtype"
)

func ReservationToInfra(res *reservation.Reservation) sqlc.CreateReservationParams {
	timeSlot := res.TimeSlot()
	tstzrange := fmt.Sprintf("[%s,%s)", timeSlot.Start().Format(time.RFC3339), timeSlot.End().Format(time.RFC3339))

	cents := res.Price().Cents()
	if cents > math.MaxInt32 || cents < math.MinInt32 {
		panic(fmt.Sprintf("price cents out of int32 range: %d", cents))
	}

	params := sqlc.CreateReservationParams{
		ResourceID: res.ResourceID(),
		UserID:     res.UserID(),
		Slot:       tstzrange,
		Status:     res.Status().String(),
		PriceCents: int32(cents),
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
