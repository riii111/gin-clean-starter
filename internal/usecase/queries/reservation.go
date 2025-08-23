package queries

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	domainerrs "gin-clean-starter/internal/pkg/errs"

	"github.com/google/uuid"
)

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

type ReservationQueries interface {
	GetByID(ctx context.Context, actor uuid.UUID, id uuid.UUID) (*ReservationView, error)
	GetByIDWithRole(ctx context.Context, actorID uuid.UUID, actorRole string, id uuid.UUID) (*ReservationView, error)
	GetByIDSystem(ctx context.Context, id uuid.UUID) (*ReservationView, error)
	ListByUser(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]*ReservationListItem, *Cursor, error)
	GenerateETag(reservation *ReservationView) string
}

func NewReservationQueries(repo ReservationReadStore) ReservationQueries {
	return &reservationQueriesImpl{repo: repo}
}

func (q *reservationQueriesImpl) GetByID(ctx context.Context, actor uuid.UUID, id uuid.UUID) (*ReservationView, error) {
	return q.GetByIDWithRole(ctx, actor, RoleViewer, id)
}

func (q *reservationQueriesImpl) GetByIDWithRole(ctx context.Context, actorID uuid.UUID, actorRole string, id uuid.UUID) (*ReservationView, error) {
	reservation, err := q.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if !canAccessReservation(actorID, actorRole, reservation) {
		// Return not found to avoid information leakage
		return nil, domainerrs.ErrReservationNotFound
	}

	return reservation, nil
}

func (q *reservationQueriesImpl) ListByUser(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]*ReservationListItem, *Cursor, error) {
	if limit <= 0 {
		limit = 50
	}

	var rows []*ReservationListItem
	var err error

	if after == nil || after.After == "" {
		rows, err = q.repo.FindByUserIDFirstPage(ctx, userID, int32(limit+1))
	} else {
		lastCreatedAt, lastID, decodeErr := decodeCursor(after.After)
		if decodeErr != nil {
			return nil, nil, decodeErr
		}
		rows, err = q.repo.FindByUserIDKeyset(ctx, userID, lastCreatedAt, lastID, int32(limit+1))
	}

	if err != nil {
		return nil, nil, err
	}

	var nextCursor *Cursor
	if len(rows) > limit {
		lastItem := rows[limit-1]
		nextCursor = &Cursor{
			After: encodeCursor(lastItem.CreatedAt, lastItem.ID),
		}
		rows = rows[:limit]
	}

	return rows, nextCursor, nil
}

func (q *reservationQueriesImpl) GenerateETag(reservation *ReservationView) string {
	return fmt.Sprintf("W/\"%s-%d\"", reservation.ID.String(), reservation.UpdatedAt.UnixNano())
}

func (q *reservationQueriesImpl) GetByIDSystem(ctx context.Context, id uuid.UUID) (*ReservationView, error) {
	return q.repo.FindByID(ctx, id)
}

func encodeCursor(createdAt time.Time, id uuid.UUID) string {
	return fmt.Sprintf("%d-%s", createdAt.UnixNano(), id.String())
}

func decodeCursor(cursor string) (time.Time, uuid.UUID, error) {
	parts := strings.SplitN(cursor, "-", 2)
	if len(parts) != 2 {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid cursor format")
	}

	timestamp, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid timestamp: %w", err)
	}

	id, err := uuid.Parse(parts[1])
	if err != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("invalid UUID: %w", err)
	}

	return time.Unix(0, timestamp), id, nil
}

func canAccessReservation(actorID uuid.UUID, actorRole string, reservation *ReservationView) bool {
	switch actorRole {
	case RoleAdmin, RoleOperator:
		return true
	case RoleViewer:
		return reservation.UserID == actorID
	default:
		return false
	}
}

type ReservationView struct {
	ID           uuid.UUID  `json:"id"`
	ResourceID   uuid.UUID  `json:"resource_id"`
	ResourceName string     `json:"resource_name"`
	UserID       uuid.UUID  `json:"user_id"`
	UserEmail    string     `json:"user_email"`
	Slot         string     `json:"slot"`
	Status       string     `json:"status"`
	PriceCents   int32      `json:"price_cents"`
	CouponID     *uuid.UUID `json:"coupon_id,omitempty"`
	CouponCode   *string    `json:"coupon_code,omitempty"`
	Note         *string    `json:"note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

type ReservationListItem struct {
	ID           uuid.UUID `json:"id"`
	ResourceID   uuid.UUID `json:"resource_id"`
	ResourceName string    `json:"resource_name"`
	Slot         string    `json:"slot"`
	Status       string    `json:"status"`
	PriceCents   int32     `json:"price_cents"`
	CreatedAt    time.Time `json:"created_at"`
}

type Cursor struct {
	After string `json:"after,omitempty"`
}

type ReservationReadStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*ReservationView, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*ReservationListItem, error)
	FindByUserIDPaginated(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*ReservationListItem, error)
	FindByUserIDFirstPage(ctx context.Context, userID uuid.UUID, limit int32) ([]*ReservationListItem, error)
	FindByUserIDKeyset(ctx context.Context, userID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32) ([]*ReservationListItem, error)
}

type reservationQueriesImpl struct {
	repo ReservationReadStore
}
