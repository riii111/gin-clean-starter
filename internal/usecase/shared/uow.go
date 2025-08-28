package shared

import (
	"context"
	"time"

	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/domain/review"
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
	// CommandReads: Direct access to command reads for validation outside transactions
	CommandReads() CommandReads
}

type Tx interface {
	Reservations() ReservationRepository
	Reviews() ReviewRepository
	RatingStats() RatingStatsRepository
	Idempotency() IdempotencyRepository
	Notifications() NotificationRepository
	Users() UserRepository
	Reads() CommandReads
	DB() sqlc.DBTX
}

type CommandReads interface {
	ResourceByID(ctx context.Context, id uuid.UUID) (*ResourceSnapshot, error)
	CouponByCode(ctx context.Context, code string) (*CouponSnapshot, error)
	ReservationByID(ctx context.Context, id uuid.UUID) (*ReservationSnapshot, error)
	IdempotencyByKey(ctx context.Context, key, userID uuid.UUID) (*IdempotencyRecord, error)
	ReviewByID(ctx context.Context, id uuid.UUID) (*ReviewSnapshot, error)
}

// Minimal snapshot for command read operations
type ReservationSnapshot struct {
	ID         uuid.UUID
	ResourceID uuid.UUID
	UserID     uuid.UUID
	Status     string
	EndTime    time.Time
}

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (uuid.UUID, error)
}

type ReviewRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, rev *review.Review) (uuid.UUID, error)
	Update(ctx context.Context, tx sqlc.DBTX, reviewID uuid.UUID, rev *review.Review) error
	Delete(ctx context.Context, tx sqlc.DBTX, reviewID uuid.UUID) error
}

type RatingStatsRepository interface {
	RecalcResourceRatingStats(ctx context.Context, tx sqlc.DBTX, resourceID uuid.UUID) error
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID, resultHash string, reservationID uuid.UUID) error
	ClaimExpiredIdempotencyKey(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID, requestHash string, expiresAt time.Time) (int64, error)
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error
}

type UserRepository interface {
	UpdateLastLogin(ctx context.Context, tx sqlc.DBTX, userID uuid.UUID) error
	Create(ctx context.Context, tx sqlc.DBTX, params sqlc.CreateUserParams) (uuid.UUID, error)
}
