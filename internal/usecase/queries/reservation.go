package queries

import (
	"context"
	"fmt"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/pkg/errs"

	"github.com/google/uuid"
)

var (
	ErrReservationNotFound = errs.New("reservation not found")
	ErrReservationAccess   = errs.New("reservation access failed")
	ErrInvalidCursor       = errs.New("invalid cursor")
)

const (
	RoleViewer   = "viewer"
	RoleOperator = "operator"
	RoleAdmin    = "admin"
)

type ReservationQueries interface {
	GetByID(ctx context.Context, actor uuid.UUID, id uuid.UUID) (*ReservationView, error)
	GetByIDWithRole(ctx context.Context, actorID uuid.UUID, actorRole string, id uuid.UUID) (*ReservationView, error)
	ListByUser(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]*ReservationListItem, *Cursor, error)
	GenerateETag(reservation *ReservationView) string
}

type ReservationReadStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*ReservationView, error)
	FindByUserIDFirstPage(ctx context.Context, userID uuid.UUID, limit int32) ([]*ReservationListItem, error)
	FindByUserIDKeyset(ctx context.Context, userID uuid.UUID, lastCreatedAt time.Time, lastID uuid.UUID, limit int32) ([]*ReservationListItem, error)
}

type reservationQueriesImpl struct {
	repo ReservationReadStore
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
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, errs.Mark(err, ErrReservationNotFound)
		}
		return nil, errs.Mark(err, ErrReservationAccess)
	}

	if !canAccessReservation(actorID, actorRole, reservation) {
		return nil, ErrReservationNotFound
	}

	return reservation, nil
}

func (q *reservationQueriesImpl) ListByUser(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]*ReservationListItem, *Cursor, error) {
	limit = ValidateLimit(limit)

	var rows []*ReservationListItem
	var err error

	if after == nil || after.After == "" {
		rows, err = q.repo.FindByUserIDFirstPage(ctx, userID, ToPgFetchLimit(limit))
	} else {
		lastCreatedAt, lastID, decodeErr := DecodeAfterCursor(after.After)
		if decodeErr != nil {
			return nil, nil, errs.Mark(decodeErr, ErrInvalidCursor)
		}
		rows, err = q.repo.FindByUserIDKeyset(ctx, userID, lastCreatedAt, lastID, ToPgFetchLimit(limit))
	}

	if err != nil {
		return nil, nil, errs.Mark(err, ErrReservationAccess)
	}

	var nextCursor *Cursor
	if len(rows) > limit {
		lastItem := rows[limit-1]
		nextCursor = &Cursor{
			After: EncodeAfterCursor(lastItem.CreatedAt, lastItem.ID),
		}
		rows = rows[:limit]
	}

	return rows, nextCursor, nil
}

func (q *reservationQueriesImpl) GenerateETag(reservation *ReservationView) string {
	return fmt.Sprintf("W/\"%s-%d\"", reservation.ID.String(), reservation.UpdatedAt.UnixMicro())
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
