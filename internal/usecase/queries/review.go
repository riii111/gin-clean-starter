package queries

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"

	"github.com/google/uuid"
)

var (
	ErrReviewNotFound = ErrReservationNotFound
	ErrReviewAccess   = ErrReservationAccess
)

type ReviewView struct {
	ID            uuid.UUID `json:"id"`
	UserID        uuid.UUID `json:"user_id"`
	UserEmail     string    `json:"user_email"`
	ResourceID    uuid.UUID `json:"resource_id"`
	ResourceName  string    `json:"resource_name"`
	ReservationID uuid.UUID `json:"reservation_id"`
	Rating        int32     `json:"rating"`
	Comment       string    `json:"comment"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type ReviewListItem struct {
	ID        uuid.UUID `json:"id"`
	UserEmail string    `json:"user_email"`
	Rating    int32     `json:"rating"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

type ResourceRatingStats struct {
	ResourceID    uuid.UUID `json:"resource_id"`
	TotalReviews  int32     `json:"total_reviews"`
	AverageRating float64   `json:"average_rating"`
	Rating1Count  int32     `json:"rating_1_count"`
	Rating2Count  int32     `json:"rating_2_count"`
	Rating3Count  int32     `json:"rating_3_count"`
	Rating4Count  int32     `json:"rating_4_count"`
	Rating5Count  int32     `json:"rating_5_count"`
	UpdatedAt     time.Time `json:"updated_at"`
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
		return nil, err
	}
	return rv, nil
}

func (q *reviewQueriesImpl) ListByResource(ctx context.Context, resourceID uuid.UUID, filters ReviewFilters, cursor *Cursor, limit int) ([]*ReviewListItem, *Cursor, error) {
	limit = ValidateLimit(limit)
	var rows []*ReviewListItem
	var err error
	if cursor == nil || cursor.After == "" {
		rows, err = q.repo.FindByResourceFirstPage(ctx, resourceID, int32(limit+1), filters.MinRating, filters.MaxRating)
	} else {
		lastCreatedAt, lastID, derr := DecodeAfterCursor(cursor.After)
		if derr != nil {
			return nil, nil, ErrInvalidCursor
		}
		rows, err = q.repo.FindByResourceKeyset(ctx, resourceID, lastCreatedAt, lastID, int32(limit+1), filters.MinRating, filters.MaxRating)
	}
	if err != nil {
		return nil, nil, err
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
		rows, err = q.repo.FindByUserFirstPage(ctx, userID, int32(limit+1))
	} else {
		lastCreatedAt, lastID, derr := DecodeAfterCursor(cursor.After)
		if derr != nil {
			return nil, nil, ErrInvalidCursor
		}
		rows, err = q.repo.FindByUserKeyset(ctx, userID, lastCreatedAt, lastID, int32(limit+1))
	}
	if err != nil {
		return nil, nil, err
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
	return q.repo.GetResourceRatingStats(ctx, resourceID)
}
