package readrepo

import (
	"context"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/pgconv"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

type ReservationViewQueries interface {
	GetReservationByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReservationByIDRow, error)
	GetReservationsByUserID(ctx context.Context, db sqlc.DBTX, userID uuid.UUID) ([]sqlc.GetReservationsByUserIDRow, error)
	GetReservationsByUserIDPaginated(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReservationsByUserIDPaginatedParams) ([]sqlc.GetReservationsByUserIDPaginatedRow, error)
}

type ReservationViewRepository struct {
	queries ReservationViewQueries
	db      sqlc.DBTX
}

func NewReservationViewRepository(queries *sqlc.Queries, db sqlc.DBTX) *ReservationViewRepository {
	return &ReservationViewRepository{
		queries: queries,
		db:      db,
	}
}

func (r *ReservationViewRepository) FindByID(ctx context.Context, id uuid.UUID) (*queries.ReservationView, error) {
	row, err := r.queries.GetReservationByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("reservation not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find reservation by ID", err)
	}

	return toReservationViewFromDetailRow(row), nil
}

func (r *ReservationViewRepository) FindByUserID(ctx context.Context, userID uuid.UUID) ([]*queries.ReservationListItem, error) {
	rows, err := r.queries.GetReservationsByUserID(ctx, r.db, userID)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find reservations by user ID", err)
	}

	result := make([]*queries.ReservationListItem, len(rows))
	for i, row := range rows {
		result[i] = toReservationListItemFromUserRow(row)
	}

	return result, nil
}

func (r *ReservationViewRepository) FindByUserIDPaginated(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*queries.ReservationListItem, error) {
	params := sqlc.GetReservationsByUserIDPaginatedParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	}

	rows, err := r.queries.GetReservationsByUserIDPaginated(ctx, r.db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find reservations by user ID with pagination", err)
	}

	result := make([]*queries.ReservationListItem, len(rows))
	for i, row := range rows {
		result[i] = toReservationListItemFromUserPaginatedRow(row)
	}

	return result, nil
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

func toReservationListItemFromUserRow(row sqlc.GetReservationsByUserIDRow) *queries.ReservationListItem {
	return &queries.ReservationListItem{
		ID:           row.ID,
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		Slot:         row.Slot,
		Status:       row.Status,
		PriceCents:   row.PriceCents,
		CreatedAt:    pgconv.TimeFromPgtype(row.CreatedAt),
	}
}

func toReservationListItemFromUserPaginatedRow(row sqlc.GetReservationsByUserIDPaginatedRow) *queries.ReservationListItem {
	return &queries.ReservationListItem{
		ID:           row.ID,
		ResourceID:   row.ResourceID,
		ResourceName: row.ResourceName,
		Slot:         row.Slot,
		Status:       row.Status,
		PriceCents:   row.PriceCents,
		CreatedAt:    pgconv.TimeFromPgtype(row.CreatedAt),
	}
}
