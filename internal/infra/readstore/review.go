package readstore

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/pgconv"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type ReviewReadQueries interface {
	GetReviewViewByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.GetReviewViewByIDRow, error)
	GetReviewsByResourceFirstPage(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReviewsByResourceFirstPageParams) ([]sqlc.GetReviewsByResourceFirstPageRow, error)
	GetReviewsByResourceKeyset(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReviewsByResourceKeysetParams) ([]sqlc.GetReviewsByResourceKeysetRow, error)
	GetReviewsByUserFirstPage(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReviewsByUserFirstPageParams) ([]sqlc.GetReviewsByUserFirstPageRow, error)
	GetReviewsByUserKeyset(ctx context.Context, db sqlc.DBTX, arg sqlc.GetReviewsByUserKeysetParams) ([]sqlc.GetReviewsByUserKeysetRow, error)
	GetResourceRatingStats(ctx context.Context, db sqlc.DBTX, resourceID uuid.UUID) (sqlc.ResourceRatingStats, error)
}

type ReviewReadStore struct {
	queries ReviewReadQueries
}

func NewReviewReadStore(queries ReviewReadQueries) *ReviewReadStore {
	return &ReviewReadStore{
		queries: queries,
	}
}

func (r *ReviewReadStore) FindByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (*queries.ReviewView, error) {
	row, err := r.queries.GetReviewViewByID(ctx, db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("review not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to get review view by id", err)
	}
	return &queries.ReviewView{
		ID:            row.ID,
		UserID:        row.UserID,
		UserEmail:     row.UserEmail,
		ResourceID:    row.ResourceID,
		ResourceName:  row.ResourceName,
		ReservationID: row.ReservationID,
		Rating:        row.Rating,
		Comment:       row.Comment,
		CreatedAt:     pgconv.TimeFromPgtype(row.CreatedAt),
		UpdatedAt:     pgconv.TimeFromPgtype(row.UpdatedAt),
	}, nil
}

func (r *ReviewReadStore) FindByResourceFirstPage(ctx context.Context, db sqlc.DBTX, resourceID uuid.UUID, limit int32, minRating, maxRating *int) ([]*queries.ReviewListItem, error) {
	params := sqlc.GetReviewsByResourceFirstPageParams{
		ResourceID: resourceID,
		Limit:      limit,
		MinRating:  toPgInt4(minRating),
		MaxRating:  toPgInt4(maxRating),
	}

	rows, err := r.queries.GetReviewsByResourceFirstPage(ctx, db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get reviews first page by resource", err)
	}
	return mapResourceFirstPageRows(rows), nil
}

func (r *ReviewReadStore) FindByResourceKeyset(ctx context.Context, db sqlc.DBTX, resourceID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32, minRating, maxRating *int) ([]*queries.ReviewListItem, error) {
	params := sqlc.GetReviewsByResourceKeysetParams{
		ResourceID: resourceID,
		CreatedAt:  pgconv.TimeToPgtype(lastCreatedAt),
		ID:         lastID,
		Limit:      limit,
		MinRating:  toPgInt4(minRating),
		MaxRating:  toPgInt4(maxRating),
	}
	rows, err := r.queries.GetReviewsByResourceKeyset(ctx, db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get reviews keyset by resource", err)
	}
	return mapResourceKeysetRows(rows), nil
}

func (r *ReviewReadStore) FindByUserFirstPage(ctx context.Context, db sqlc.DBTX, userID uuid.UUID, limit int32) ([]*queries.ReviewListItem, error) {
	params := sqlc.GetReviewsByUserFirstPageParams{UserID: userID, Limit: limit}
	rows, err := r.queries.GetReviewsByUserFirstPage(ctx, db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get reviews first page by user", err)
	}
	return mapUserFirstPageRows(rows), nil
}

func (r *ReviewReadStore) FindByUserKeyset(ctx context.Context, db sqlc.DBTX, userID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32) ([]*queries.ReviewListItem, error) {
	params := sqlc.GetReviewsByUserKeysetParams{
		UserID:    userID,
		CreatedAt: pgconv.TimeToPgtype(lastCreatedAt),
		ID:        lastID,
		Limit:     limit,
	}
	rows, err := r.queries.GetReviewsByUserKeyset(ctx, db, params)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get reviews keyset by user", err)
	}
	return mapUserKeysetRows(rows), nil
}

func (r *ReviewReadStore) GetResourceRatingStats(ctx context.Context, db sqlc.DBTX, resourceID uuid.UUID) (*queries.ResourceRatingStats, error) {
	row, err := r.queries.GetResourceRatingStats(ctx, db, resourceID)
	if err != nil {
		if pgconv.IsNoRows(err) {
			// return zero stats if not initialized yet
			return &queries.ResourceRatingStats{ResourceID: resourceID}, nil
		}
		return nil, infra.WrapRepoErr("failed to get resource rating stats", err)
	}
	avgPtr, _ := pgconv.Float64PtrFromNumeric(row.AverageRating)
	avg := 0.0
	if avgPtr != nil {
		avg = *avgPtr
	}
	return &queries.ResourceRatingStats{
		ResourceID:    row.ResourceID,
		TotalReviews:  row.TotalReviews,
		AverageRating: avg,
		Rating1Count:  row.Rating1Count,
		Rating2Count:  row.Rating2Count,
		Rating3Count:  row.Rating3Count,
		Rating4Count:  row.Rating4Count,
		Rating5Count:  row.Rating5Count,
		UpdatedAt:     pgconv.TimeFromPgtype(row.UpdatedAt),
	}, nil
}

// FindSnapshotByID returns a minimal review snapshot for command use cases.
func (r *ReviewReadStore) FindSnapshotByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (*shared.ReviewSnapshot, error) {
	row, err := r.queries.GetReviewViewByID(ctx, db, id)
	if err != nil {
		if pgconv.IsNoRows(err) {
			return nil, infra.WrapRepoErr("review not found", err, infra.KindNotFound)
		}
		return nil, infra.WrapRepoErr("failed to get review view by id", err)
	}
	return &shared.ReviewSnapshot{
		ID:            row.ID,
		UserID:        row.UserID,
		ResourceID:    row.ResourceID,
		ReservationID: row.ReservationID,
		Rating:        int(row.Rating),
		Comment:       row.Comment,
	}, nil
}

func toPgInt4(v *int) pgtype.Int4 {
	if v == nil {
		return pgtype.Int4{Valid: false}
	}
	return pgtype.Int4{Int32: pgconv.IntToInt32(*v), Valid: true}
}

func mapResourceFirstPageRows(rows []sqlc.GetReviewsByResourceFirstPageRow) []*queries.ReviewListItem {
	result := make([]*queries.ReviewListItem, len(rows))
	for i, row := range rows {
		result[i] = &queries.ReviewListItem{
			ID:        row.ID,
			UserEmail: row.UserEmail,
			Rating:    row.Rating,
			Comment:   row.Comment,
			CreatedAt: pgconv.TimeFromPgtype(row.CreatedAt),
		}
	}
	return result
}

func mapResourceKeysetRows(rows []sqlc.GetReviewsByResourceKeysetRow) []*queries.ReviewListItem {
	result := make([]*queries.ReviewListItem, len(rows))
	for i, row := range rows {
		result[i] = &queries.ReviewListItem{
			ID:        row.ID,
			UserEmail: row.UserEmail,
			Rating:    row.Rating,
			Comment:   row.Comment,
			CreatedAt: pgconv.TimeFromPgtype(row.CreatedAt),
		}
	}
	return result
}

func mapUserFirstPageRows(rows []sqlc.GetReviewsByUserFirstPageRow) []*queries.ReviewListItem {
	result := make([]*queries.ReviewListItem, len(rows))
	for i, row := range rows {
		result[i] = &queries.ReviewListItem{
			ID:        row.ID,
			UserEmail: row.UserEmail,
			Rating:    row.Rating,
			Comment:   row.Comment,
			CreatedAt: pgconv.TimeFromPgtype(row.CreatedAt),
		}
	}
	return result
}

func mapUserKeysetRows(rows []sqlc.GetReviewsByUserKeysetRow) []*queries.ReviewListItem {
	result := make([]*queries.ReviewListItem, len(rows))
	for i, row := range rows {
		result[i] = &queries.ReviewListItem{
			ID:        row.ID,
			UserEmail: row.UserEmail,
			Rating:    row.Rating,
			Comment:   row.Comment,
			CreatedAt: pgconv.TimeFromPgtype(row.CreatedAt),
		}
	}
	return result
}
