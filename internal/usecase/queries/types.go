package queries

import (
	"time"

	"github.com/google/uuid"
)

// AuthorizedUserView represents read-optimized user data with authorization info
type AuthorizedUserView struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	CompanyID *uuid.UUID `json:"company_id,omitempty"`
	IsActive  bool       `json:"is_active"`
}

// IdempotencyKeyView represents read-optimized idempotency key data
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

// NotificationJobView represents read-optimized notification job data
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
