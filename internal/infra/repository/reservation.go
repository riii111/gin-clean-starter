package repository

import (
	"context"

	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/repository/converter"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type ReservationWriteQueries interface {
	CreateReservation(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateReservationParams) (sqlc.Reservations, error)
	GetReservationByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReservationByIDRow, error)
}

type ReservationRepository struct {
	queries ReservationWriteQueries
	db      sqlc.DBTX
}

func NewReservationRepository(queries *sqlc.Queries, db sqlc.DBTX) *ReservationRepository {
	return &ReservationRepository{
		queries: queries,
		db:      db,
	}
}

func (r *ReservationRepository) Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (uuid.UUID, error) {
	params := converter.ReservationToInfra(res)

	result, err := r.queries.CreateReservation(ctx, tx, params)
	if err != nil {
		return uuid.Nil, infra.WrapRepoErr("failed to create reservation", err)
	}

	return result.ID, nil
}
