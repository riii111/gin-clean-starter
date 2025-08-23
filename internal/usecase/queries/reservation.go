package queries

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Read models (DTO for read side)
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

type ResourceView struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	LeadTimeMin int32     `json:"lead_time_min"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CouponView struct {
	ID             uuid.UUID  `json:"id"`
	Code           string     `json:"code"`
	AmountOffCents *int32     `json:"amount_off_cents,omitempty"`
	PercentOff     *float64   `json:"percent_off,omitempty"`
	ValidFrom      *time.Time `json:"valid_from,omitempty"`
	ValidTo        *time.Time `json:"valid_to,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type AuthorizedUserView struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	CompanyID *uuid.UUID `json:"company_id,omitempty"`
	IsActive  bool       `json:"is_active"`
}

type IdempotencyKeyView struct {
	Key                 uuid.UUID  `json:"key"`
	UserID              uuid.UUID  `json:"user_id"`
	Endpoint            string     `json:"endpoint"`
	RequestHash         string     `json:"request_hash"`
	ResponseBodyHash    *string    `json:"response_body_hash,omitempty"`
	Status              string     `json:"status"`
	ResultReservationID *uuid.UUID `json:"result_reservation_id,omitempty"`
	ExpiresAt           time.Time  `json:"expires_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

type NotificationJobView struct {
	ID        uuid.UUID `json:"id"`
	Kind      string    `json:"kind"`
	Topic     string    `json:"topic"`
	Payload   []byte    `json:"payload"`
	RunAt     time.Time `json:"run_at"`
	Attempts  int32     `json:"attempts"`
	Status    string    `json:"status"`
	LastError *string   `json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Cursor struct {
	After string
}

type ReservationQueries interface {
	GetByID(ctx context.Context, actor uuid.UUID, id uuid.UUID) (*ReservationView, error)
	ListByUser(ctx context.Context, userID uuid.UUID, after *Cursor, limit int) ([]*ReservationListItem, *Cursor, error)
}

type ReservationViewRepo interface {
	FindByID(ctx context.Context, id uuid.UUID) (*ReservationView, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*ReservationListItem, error)
	FindByUserIDPaginated(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]*ReservationListItem, error)
}

type reservationQueriesImpl struct {
	repo ReservationViewRepo
}

func NewReservationQueries(repo ReservationViewRepo) ReservationQueries {
	return &reservationQueriesImpl{repo: repo}
}

func (q *reservationQueriesImpl) GetByID(ctx context.Context, _ uuid.UUID, id uuid.UUID) (*ReservationView, error) {
	return q.repo.FindByID(ctx, id)
}

func (q *reservationQueriesImpl) ListByUser(ctx context.Context, userID uuid.UUID, _ *Cursor, limit int) ([]*ReservationListItem, *Cursor, error) {
	if limit <= 0 {
		limit = 50
	}
	// Future: implement real keyset encoding/decoding in Cursor
	rows, err := q.repo.FindByUserIDPaginated(ctx, userID, int32(limit), 0)
	if err != nil {
		return nil, nil, err
	}
	return rows, nil, nil
}
