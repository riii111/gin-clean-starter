package writerepo

import (
	"context"

	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/converter"
	"gin-clean-starter/internal/infra/pgconv"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

type ReservationQueries interface {
	CreateReservation(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateReservationParams) (sqlc.Reservations, error)
	GetReservationByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReservationByIDRow, error)
}

type ReservationRepository struct {
	queries ReservationQueries
	db      sqlc.DBTX
}

func NewReservationRepository(queries *sqlc.Queries, db sqlc.DBTX) *ReservationRepository {
	return &ReservationRepository{
		queries: queries,
		db:      db,
	}
}

func (r *ReservationRepository) Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (*queries.ReservationView, error) {
	params := converter.ReservationToInfra(res)

	result, err := r.queries.CreateReservation(ctx, tx, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to create reservation", err)
	}

	detailRow, err := r.queries.GetReservationByID(ctx, tx, result.ID)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get created reservation", err)
	}

	return toReservationViewFromDetailRow(detailRow), nil
}

func toReservationViewFromDetailRow(row sqlc.GetReservationByIDRow) *queries.ReservationView {
	return &queries.ReservationView{
		ID:           row.ID,
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		UserID:       row.UserID,
		UserEmail:    row.UserEmail,
		Slot:         row.Slot,
		Status:       row.Status,
		PriceCents:   row.PriceCents,
		CouponID:     pgconv.UUIDPtrFromPgtype(row.CouponID),
		CouponCode:   pgconv.StringPtrFromPgtype(row.CouponCode),
		Note:         pgconv.StringPtrFromPgtype(row.Note),
		CreatedAt:    pgconv.TimeFromPgtype(row.CreatedAt),
		UpdatedAt:    pgconv.TimeFromPgtype(row.UpdatedAt),
	}
}
