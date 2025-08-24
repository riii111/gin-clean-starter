package readstore

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
)

type ReservationViewQueries interface {
	GetReservationByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReservationByIDRow, error)
	GetReservationsByUserIDFirstPage(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReservationsByUserIDFirstPageParams) ([]sqlc.GetReservationsByUserIDFirstPageRow, error)
	GetReservationsByUserIDKeyset(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReservationsByUserIDKeysetParams) ([]sqlc.GetReservationsByUserIDKeysetRow, error)
}

type ReservationStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*queries.ReservationView, error)
}

type ReservationReadStore struct {
	queries ReservationViewQueries
	db      sqlc.DBTX
}

func NewReservationReadStore(queries *sqlc.Queries, db sqlc.DBTX) *ReservationReadStore {
	return &ReservationReadStore{
		queries: queries,
		db:      db,
	}
}

func (r *ReservationReadStore) FindByID(ctx context.Context, id uuid.UUID) (*queries.ReservationView, error) {
	row, err := r.queries.GetReservationByID(ctx, r.db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("reservation not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to find reservation by ID", err)
	}

	return rowToReservationView(row), nil
}

func rowToReservationView(row sqlc.GetReservationByIDRow) *queries.ReservationView {
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

func (r *ReservationReadStore) FindByUserIDFirstPage(ctx context.Context, userID uuid.UUID, limit int32) ([]*queries.ReservationListItem, error) {
	params := sqlc.GetReservationsByUserIDFirstPageParams{
		UserID: userID,
		Limit:  limit,
	}

	rows, err := r.queries.GetReservationsByUserIDFirstPage(ctx, r.db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find reservations first page", err)
	}

	result := make([]*queries.ReservationListItem, len(rows))
	for i, row := range rows {
		result[i] = toReservationListItemFromUserFirstPageRow(row)
	}

	return result, nil
}

func (r *ReservationReadStore) FindByUserIDKeyset(ctx context.Context, userID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32) ([]*queries.ReservationListItem, error) {
	params := sqlc.GetReservationsByUserIDKeysetParams{
		UserID:    userID,
		CreatedAt: pgconv.TimeToPgtype(lastCreatedAt),
		ID:        lastID,
		Limit:     limit,
	}

	rows, err := r.queries.GetReservationsByUserIDKeyset(ctx, r.db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to find reservations keyset", err)
	}

	result := make([]*queries.ReservationListItem, len(rows))
	for i, row := range rows {
		result[i] = toReservationListItemFromUserKeysetRow(row)
	}

	return result, nil
}

func toReservationListItemFromUserFirstPageRow(row sqlc.GetReservationsByUserIDFirstPageRow) *queries.ReservationListItem {
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

func toReservationListItemFromUserKeysetRow(row sqlc.GetReservationsByUserIDKeysetRow) *queries.ReservationListItem {
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
