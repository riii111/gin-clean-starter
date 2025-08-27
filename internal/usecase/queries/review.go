package queries

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/pkg/pgconv"

	"github.com/google/uuid"
)

var (
	ErrReviewNotFound     = errs.New("review not found")
	ErrReviewAccess       = errs.New("review access denied")
	ErrReviewQueryFailed  = errs.New("review query failed")
	ErrInvalidCursorQuery = errs.New("invalid cursor for review query")
)

type ReviewView struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"userId"`
	UserEmail     string    `json:"userEmail"`
	ResourceID    uuid.UUID `json:"resourceId"`
	ResourceName  string    `json:"resourceName"`
	ReservationID uuid.UUID `json:"reservationId"`
	Rating        int32     `json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ReviewListItem struct {
	ID        uuid.UUID `json:"id"`
	UserEmail string    `json:"userEmail"`
	Rating    int32     `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"createdAt"`
}

type ResourceRatingStats struct {
	ResourceID    uuid.UUID `json:"resourceId"`
	TotalReviews  int32     `json:"totalReviews"`
	AverageRating float64   `json:"averageRating"`
	Rating1Count  int32     `json:"rating1Count"`
	Rating2Count  int32     `json:"rating2Count"`
	Rating3Count  int32     `json:"rating3Count"`
	Rating4Count  int32     `json:"rating4Count"`
	Rating5Count  int32     `json:"rating5Count"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

type ReviewFilters struct {
	MinRating *int
	MaxRating *int
}

type ReviewReadStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*ReviewView, error)
	FindByResourceFirstPage(ctx context.Context, resourceID uuid.UUID, limit int32, minRating, maxRating *int) ([]*ReviewListItem, error)
	FindByResourceKeyset(ctx context.Context, resourceID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32, minRating, maxRating *int) ([]*ReviewListItem, error)
	FindByUserFirstPage(ctx context.Context, userID uuid.UUID, limit int32) ([]*ReviewListItem, error)
	FindByUserKeyset(ctx context.Context, userID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32) ([]*ReviewListItem, error)
	GetResourceRatingStats(ctx context.Context, resourceID uuid.UUID) (*ResourceRatingStats, error)
}

type ReviewQueries interface {
	GetByID(ctx context.Context, id uuid.UUID) (*ReviewView, error)
	ListByResource(ctx context.Context, resourceID uuid.UUID, filters ReviewFilters, cursor *Cursor, limit int) ([]*ReviewListItem, *Cursor, error)
	ListByUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID, actorRole string, cursor *Cursor, limit int) ([]*ReviewListItem, *Cursor, error)
	GetResourceRatingStats(ctx context.Context, resourceID uuid.UUID) (*ResourceRatingStats, error)
}

type reviewQueriesImpl struct {
	repo ReviewReadStore
}

func NewReviewQueries(repo ReviewReadStore) ReviewQueries {
	return &reviewQueriesImpl{repo: repo}
}

func (q *reviewQueriesImpl) GetByID(ctx context.Context, id uuid.UUID) (*ReviewView, error) {
	rv, err := q.repo.FindByID(ctx, id)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, errs.Mark(err, ErrReviewQueryFailed)
	}
	return rv, nil
}

func (q *reviewQueriesImpl) ListByResource(ctx context.Context, resourceID uuid.UUID, filters ReviewFilters, cursor *Cursor, limit int) ([]*ReviewListItem, *Cursor, error) {
	limit = ValidateLimit(limit)
	var rows []*ReviewListItem
	var err error
	if cursor == nil || cursor.After == "" {
		limit32 := pgconv.IntToInt32(limit + 1)
		rows, err = q.repo.FindByResourceFirstPage(ctx, resourceID, limit32, filters.MinRating, filters.MaxRating)
	} else {
		lastCreatedAt, lastID, derr := DecodeAfterCursor(cursor.After)
		if derr != nil {
			return nil, nil, errs.Mark(derr, ErrInvalidCursorQuery)
		}
		limit32 := pgconv.IntToInt32(limit + 1)
		rows, err = q.repo.FindByResourceKeyset(ctx, resourceID, lastCreatedAt, lastID, limit32, filters.MinRating, filters.MaxRating)
	}
	if err != nil {
		return nil, nil, errs.Mark(err, ErrReviewQueryFailed)
	}
	var next *Cursor
	if len(rows) > limit {
		last := rows[limit-1]
		next = &Cursor{After: EncodeAfterCursor(last.CreatedAt, last.ID)}
		rows = rows[:limit]
	}
	return rows, next, nil
}

func (q *reviewQueriesImpl) ListByUser(ctx context.Context, userID uuid.UUID, actorID uuid.UUID, actorRole string, cursor *Cursor, limit int) ([]*ReviewListItem, *Cursor, error) {
	switch actorRole {
	case RoleAdmin, RoleOperator:
	case RoleViewer:
		if userID != actorID {
			return nil, nil, ErrReviewAccess
		}
	default:
		return nil, nil, ErrReviewAccess
	}

	limit = ValidateLimit(limit)
	var rows []*ReviewListItem
	var err error
	if cursor == nil || cursor.After == "" {
		limit32 := pgconv.IntToInt32(limit + 1)
		rows, err = q.repo.FindByUserFirstPage(ctx, userID, limit32)
	} else {
		lastCreatedAt, lastID, derr := DecodeAfterCursor(cursor.After)
		if derr != nil {
			return nil, nil, errs.Mark(derr, ErrInvalidCursorQuery)
		}
		limit32 := pgconv.IntToInt32(limit + 1)
		rows, err = q.repo.FindByUserKeyset(ctx, userID, lastCreatedAt, lastID, limit32)
	}
	if err != nil {
		return nil, nil, errs.Mark(err, ErrReviewQueryFailed)
	}
	var next *Cursor
	if len(rows) > limit {
		last := rows[limit-1]
		next = &Cursor{After: EncodeAfterCursor(last.CreatedAt, last.ID)}
		rows = rows[:limit]
	}
	return rows, next, nil
}

func (q *reviewQueriesImpl) GetResourceRatingStats(ctx context.Context, resourceID uuid.UUID) (*ResourceRatingStats, error) {
	stats, err := q.repo.GetResourceRatingStats(ctx, resourceID)
	if err != nil {
		return nil, errs.Mark(err, ErrReviewQueryFailed)
	}
	return stats, nil
}
