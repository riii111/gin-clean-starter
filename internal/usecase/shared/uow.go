package shared

import (
	"context"
	"time"

	"gin-clean-starter/internal/domain/reservation"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
)

type UnitOfWork interface {
	// Within: Full transaction for write operations with retry logic
	Within(ctx context.Context, fn func(ctx context.Context, tx Tx) error) error
	// WithinReadOnly: Read-only transaction for multi-table consistent reads
	WithinReadOnly(ctx context.Context, fn func(ctx context.Context, db sqlc.DBTX) error) error
	// WithDB: Single query operations using implicit transactions
	WithDB(ctx context.Context, fn func(ctx context.Context, db sqlc.DBTX) error) error
}

type Tx interface {
	Reservations() ReservationRepository
	Idempotency() IdempotencyRepository
	Notifications() NotificationRepository
	Reads() CommandReads
	DB() sqlc.DBTX
}

type CommandReads interface {
	ResourceByID(ctx context.Context, id uuid.UUID) (*ResourceSnapshot, error)
	CouponByCode(ctx context.Context, code string) (*CouponSnapshot, error)
	ReservationByID(ctx context.Context, id uuid.UUID) (*ReservationSnapshot, error)
	IdempotencyByKey(ctx context.Context, key, userID uuid.UUID) (*IdempotencyRecord, error)
}

// Minimal snapshot for command read operations
type ReservationSnapshot struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	UserID     uuid.UUID
	Status     string
}

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (uuid.UUID, error)
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID, resultHash string, reservationID uuid.UUID) error
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error
}
