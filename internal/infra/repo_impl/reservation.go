package repo_impl

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/converter"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReservationQueries interface {
	CreateReservation(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateReservationParams) (sqlc.Reservations, error)
	GetReservationByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReservationByIDRow, error)
	GetReservationsByUserID(ctx context.Context, db sqlc.DBTX, userID uuid.UUID) ([]sqlc.GetReservationsByUserIDRow, error)
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

func (r *ReservationRepository) Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (*readmodel.ReservationRM, error) {
	params := converter.ReservationToInfra(res)

	result, err := r.queries.CreateReservation(ctx, tx, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to create reservation", err)
	}

	detailRow, err := r.queries.GetReservationByID(ctx, tx, result.ID)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get created reservation", err)
	}

	return toReservationRMFromDetailRow(detailRow), nil
}

func (r *ReservationRepository) FindByID(ctx context.Context, id uuid.UUID) (*readmodel.ReservationRM, error) {
	row, err := r.queries.GetReservationByID(ctx, r.db, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, infra.WrapRepoErr("reservation not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find reservation by ID", err)
	}

	return toReservationRMFromDetailRow(row), nil
}

func (r *ReservationRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*readmodel.ReservationListRM, error) {
	rows, err := r.queries.GetReservationsByUserID(ctx, r.db, userID)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find reservations by user ID", err)
	}

	result := make([]*readmodel.ReservationListRM, len(rows))
	for i, row := range rows {
		result[i] = toReservationListRMFromUserRow(row)
	}

	return result, nil
}

func toReservationRMFromDetailRow(row sqlc.GetReservationByIDRow) *readmodel.ReservationRM {
	return &readmodel.ReservationRM{
		ID:           row.ID,
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		UserID:       row.UserID,
		UserEmail:    row.UserEmail,
		Slot:         row.Slot,
		Status:       row.Status,
		PriceCents:   row.PriceCents,
		CouponID:     pgtypeToUUIDPtr(row.CouponID),
		CouponCode:   pgtypeToStringPtr(row.CouponCode),
		Note:         pgtypeToStringPtr(row.Note),
		CreatedAt:    pgtypeToTime(row.CreatedAt),
		UpdatedAt:    pgtypeToTime(row.UpdatedAt),
	}
}

func toReservationListRMFromUserRow(row sqlc.GetReservationsByUserIDRow) *readmodel.ReservationListRM {
	return &readmodel.ReservationListRM{
		ID:           row.ID,
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		Slot:         row.Slot,
		Status:       row.Status,
		PriceCents:   row.PriceCents,
		CreatedAt:    pgtypeToTime(row.CreatedAt),
	}
}

func pgtypeToUUIDPtr(pu pgtype.UUID) *uuid.UUID {
	if !pu.Valid {
		return nil
	}
	id := uuid.UUID(pu.Bytes)
	return &id
}

func pgtypeToStringPtr(pt pgtype.Text) *string {
	if !pt.Valid {
		return nil
	}
	return &pt.String
}

func pgtypeToTime(pt pgtype.Timestamptz) time.Time {
	return pt.Time
}
