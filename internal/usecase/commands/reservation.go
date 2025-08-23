package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/domain/resource"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrResourceNotFound        = errs.New("resource not found")
	ErrCouponNotFound          = errs.New("coupon not found")
	ErrInvalidTimeSlot         = errs.New("invalid time slot")
	ErrInsufficientLeadTime    = errs.New("insufficient lead time")
	ErrDuplicateReservation    = errs.New("duplicate reservation")
	ErrReservationConflict     = errs.New("reservation conflict")
	ErrInvalidCoupon           = errs.New("invalid coupon")
	ErrIdempotencyInProgress   = errs.New("idempotency in progress")
	ErrDomainValidation        = errs.New("domain validation error")
	ErrIdempotencyCheckFailed  = errs.New("idempotency check failed")
	ErrDatabaseOperationFailed = errs.New("database operation failed")
)

type CreateReservationResult struct {
	Reservation *queries.ReservationView
	IsReplayed  bool
}

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (uuid.UUID, error)
}

type ResourceRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*ResourceSnapshot, error)
}

type CouponRepository interface {
	FindByCode(ctx context.Context, code string) (*CouponSnapshot, error)
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	Get(ctx context.Context, key uuid.UUID, userID uuid.UUID) (*IdempotencyRecord, error)
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, responseBodyHash string, resultReservationID uuid.UUID) error
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error
}

type ReservationCommands interface {
	CreateReservation(ctx context.Context, req reqdto.CreateReservationRequest, userID uuid.UUID, idempotencyKey uuid.UUID) (*CreateReservationResult, error)
}

type reservationUseCaseImpl struct {
	reservationRepo    ReservationRepository
	resourceRepo       ResourceRepository
	couponRepo         CouponRepository
	idempotencyRepo    IdempotencyRepository
	notificationRepo   NotificationRepository
	reservationFactory *reservation.Factory
	reservationQueries queries.ReservationQueries
	db                 *pgxpool.Pool
	clock              clock.Clock
}

func NewReservationUseCase(
	reservationRepo ReservationRepository,
	resourceRepo ResourceRepository,
	couponRepo CouponRepository,
	idempotencyRepo IdempotencyRepository,
	notificationRepo NotificationRepository,
	reservationFactory *reservation.Factory,
	reservationQueries queries.ReservationQueries,
	db *pgxpool.Pool,
	clock clock.Clock,
) ReservationCommands {
	return &reservationUseCaseImpl{
		reservationRepo:    reservationRepo,
		resourceRepo:       resourceRepo,
		couponRepo:         couponRepo,
		idempotencyRepo:    idempotencyRepo,
		notificationRepo:   notificationRepo,
		reservationFactory: reservationFactory,
		reservationQueries: reservationQueries,
		db:                 db,
		clock:              clock,
	}
}

func (r *reservationUseCaseImpl) CreateReservation(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	userID uuid.UUID,
	idempotencyKey uuid.UUID,
) (*CreateReservationResult, error) {
	requestHash := r.calculateRequestHash(req)
	expiresAt := r.clock.Now().Add(24 * time.Hour)

	existingResult, err := r.handleIdempotency(ctx, idempotencyKey, userID, requestHash, expiresAt)
	if err != nil {
		return nil, err
	}
	if existingResult != nil {
		return &CreateReservationResult{
			Reservation: existingResult,
			IsReplayed:  true,
		}, nil
	}

	reservationView, err := r.createNewReservation(ctx, req, userID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	return &CreateReservationResult{
		Reservation: reservationView,
		IsReplayed:  false,
	}, nil
}

func (r *reservationUseCaseImpl) handleIdempotency(
	ctx context.Context,
	idempotencyKey, userID uuid.UUID,
	requestHash string,
	expiresAt time.Time,
) (*queries.ReservationView, error) {
	if err := r.idempotencyRepo.TryInsert(ctx, idempotencyKey, userID, "POST /reservations", requestHash, expiresAt); err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	existing, err := r.idempotencyRepo.Get(ctx, idempotencyKey, userID)
	if err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	switch existing.Status {
	case "completed":
		if existing.ResultReservationID != nil {
			// Use system-level access for idempotency replay
			return r.reservationQueries.GetByIDSystem(ctx, *existing.ResultReservationID)
		}
		return nil, errs.New("completed request missing result reservation ID")

	case "processing":
		if existing.RequestHash != requestHash {
			return nil, ErrDuplicateReservation
		}
		return nil, ErrIdempotencyInProgress

	default:
		return nil, errs.New("invalid idempotency key status")
	}
}

func (r *reservationUseCaseImpl) createNewReservation(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	userID, idempotencyKey uuid.UUID,
) (*queries.ReservationView, error) {
	resourceEntity, err := r.validateAndGetResource(ctx, req.ResourceID)
	if err != nil {
		return nil, err
	}

	couponEntity, err := r.validateAndGetCoupon(ctx, req.GetCouponCode())
	if err != nil {
		return nil, err
	}

	domainData, err := req.ToDomain()
	if err != nil {
		return nil, errs.Mark(err, ErrInvalidTimeSlot)
	}

	reservationEntity, err := r.reservationFactory.CreateReservation(
		resourceEntity,
		userID,
		domainData.TimeSlot,
		couponEntity,
		domainData.Note,
	)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidation)
	}

	return r.executeReservationTransaction(ctx, reservationEntity, idempotencyKey, userID)
}

func (r *reservationUseCaseImpl) executeReservationTransaction(
	ctx context.Context,
	reservationEntity *reservation.Reservation,
	idempotencyKey, userID uuid.UUID,
) (*queries.ReservationView, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			slog.Warn("failed to rollback transaction", "error", rollbackErr)
		}
	}()

	reservationID, err := r.reservationRepo.Create(ctx, tx, reservationEntity)
	if err != nil {
		if infra.IsKind(err, infra.KindConflict) {
			return nil, ErrReservationConflict
		}
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if notificationErr := r.createNotificationJobByID(ctx, tx, reservationID); notificationErr != nil {
		return nil, errs.Mark(notificationErr, ErrDatabaseOperationFailed)
	}

	// Placeholder for response hash until we read the full data
	tempHash := r.calculateIDHash(reservationID)
	err = r.idempotencyRepo.UpdateStatusCompleted(ctx, tx, idempotencyKey, userID, tempHash, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if commitErr := tx.Commit(ctx); commitErr != nil {
		return nil, errs.Mark(commitErr, ErrDatabaseOperationFailed)
	}

	// Read-after-write: Get the complete reservation view from read store
	reservationView, err := r.reservationQueries.GetByIDSystem(ctx, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	return reservationView, nil
}

func (r *reservationUseCaseImpl) validateAndGetResource(
	ctx context.Context,
	resourceID uuid.UUID,
) (*resource.Resource, error) {
	resourceRM, err := r.resourceRepo.FindByID(ctx, resourceID)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, errs.Mark(err, ErrResourceNotFound)
	}

	return resource.NewResource(resourceRM.ID, resourceRM.Name, resourceRM.LeadTimeMin)
}

func (r *reservationUseCaseImpl) validateAndGetCoupon(
	ctx context.Context,
	couponCode *string,
) (*coupon.Coupon, error) {
	if couponCode == nil {
		return nil, nil
	}

	couponRM, err := r.couponRepo.FindByCode(ctx, *couponCode)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrCouponNotFound
		}
		return nil, errs.Mark(err, ErrCouponNotFound)
	}

	couponEntity, err := coupon.NewCoupon(
		couponRM.ID,
		couponRM.Code,
		couponRM.AmountOffCents,
		couponRM.PercentOff,
		couponRM.ValidFrom,
		couponRM.ValidTo,
	)
	if err != nil {
		return nil, errs.Mark(err, ErrInvalidCoupon)
	}

	if err := couponEntity.ValidateUsage(r.clock.Now()); err != nil {
		return nil, ErrInvalidCoupon
	}

	return couponEntity, nil
}

func (r *reservationUseCaseImpl) createNotificationJobByID(
	ctx context.Context,
	tx sqlc.DBTX,
	reservationID uuid.UUID,
) error {
	// Simple notification job with minimal data until we read the full reservation
	notificationPayload, err := json.Marshal(map[string]any{
		"reservation_id": reservationID,
		"type":           "reservation_created",
	})
	if err != nil {
		return err
	}

	return r.notificationRepo.CreateJob(ctx, tx, "email", "reservation_created", notificationPayload, r.clock.Now())
}

func (r *reservationUseCaseImpl) calculateRequestHash(req reqdto.CreateReservationRequest) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (r *reservationUseCaseImpl) calculateIDHash(id uuid.UUID) string {
	hash := sha256.Sum256([]byte(id.String()))
	return hex.EncodeToString(hash[:])
}
