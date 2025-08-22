package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"gin-clean-starter/internal/domain/coupon"
	"gin-clean-starter/internal/domain/reservation"
	"gin-clean-starter/internal/domain/resource"
	reqdto "gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/readmodel"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrReservationNotFound    = errors.New("reservation not found")
	ErrResourceNotFound       = errors.New("resource not found")
	ErrCouponNotFound         = errors.New("coupon not found")
	ErrInvalidTimeSlot        = errors.New("invalid time slot")
	ErrInsufficientLeadTime   = errors.New("insufficient lead time")
	ErrDuplicateReservation   = errors.New("duplicate reservation request")
	ErrReservationConflict    = errors.New("time slot conflict")
	ErrIdempotencyKeyRequired = errors.New("idempotency-key header required")
	ErrInvalidCoupon          = errors.New("invalid or expired coupon")

	// Error markers for categorization
	ErrDomainValidationFailed  = errors.New("domain validation failed")
	ErrIdempotencyCheckFailed  = errors.New("idempotency check failed")
	ErrDatabaseOperationFailed = errors.New("database operation failed")
)

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (*readmodel.ReservationRM, error)
	FindByID(ctx context.Context, id uuid.UUID) (*readmodel.ReservationRM, error)
	FindByUserID(ctx context.Context, userID uuid.UUID) ([]*readmodel.ReservationListRM, error)
}

type ResourceRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*readmodel.ResourceRM, error)
}

type CouponRepository interface {
	FindByCode(ctx context.Context, code string) (*readmodel.CouponRM, error)
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	Get(ctx context.Context, key uuid.UUID, userID uuid.UUID) (*readmodel.IdempotencyKeyRM, error)
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, responseBodyHash string, resultReservationID uuid.UUID) error
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind string, payload []byte, runAt time.Time) error
}

type ReservationUseCase interface {
	CreateReservation(ctx context.Context, req reqdto.CreateReservationRequest, userID uuid.UUID, idempotencyKey uuid.UUID) (*readmodel.ReservationRM, error)
	GetReservation(ctx context.Context, id uuid.UUID) (*readmodel.ReservationRM, error)
	GetUserReservations(ctx context.Context, userID uuid.UUID) ([]*readmodel.ReservationListRM, error)
}

type reservationUseCaseImpl struct {
	reservationRepo  ReservationRepository
	resourceRepo     ResourceRepository
	couponRepo       CouponRepository
	idempotencyRepo  IdempotencyRepository
	notificationRepo NotificationRepository
	db               *pgxpool.Pool
	clock            clock.Clock
}

func NewReservationUseCase(
	reservationRepo ReservationRepository,
	resourceRepo ResourceRepository,
	couponRepo CouponRepository,
	idempotencyRepo IdempotencyRepository,
	notificationRepo NotificationRepository,
	db *pgxpool.Pool,
	clock clock.Clock,
) ReservationUseCase {
	return &reservationUseCaseImpl{
		reservationRepo:  reservationRepo,
		resourceRepo:     resourceRepo,
		couponRepo:       couponRepo,
		idempotencyRepo:  idempotencyRepo,
		notificationRepo: notificationRepo,
		db:               db,
		clock:            clock,
	}
}

func (r *reservationUseCaseImpl) CreateReservation(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	userID uuid.UUID,
	idempotencyKey uuid.UUID,
) (*readmodel.ReservationRM, error) {
	requestHash := r.calculateRequestHash(req)
	expiresAt := r.clock.Now().Add(24 * time.Hour)

	existingResult, err := r.handleIdempotency(ctx, idempotencyKey, userID, requestHash, expiresAt)
	if err != nil {
		return nil, err
	}
	if existingResult != nil {
		return existingResult, nil
	}

	return r.createNewReservation(ctx, req, userID, idempotencyKey)
}

func (r *reservationUseCaseImpl) handleIdempotency(
	ctx context.Context,
	idempotencyKey, userID uuid.UUID,
	requestHash string,
	expiresAt time.Time,
) (*readmodel.ReservationRM, error) {
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
			return r.reservationRepo.FindByID(ctx, *existing.ResultReservationID)
		}
		return nil, errs.New("completed request missing result reservation ID")

	case "processing":
		if existing.RequestHash != requestHash {
			return nil, ErrDuplicateReservation
		}
		return nil, nil

	default:
		return nil, errs.New("invalid idempotency key status")
	}
}

func (r *reservationUseCaseImpl) createNewReservation(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	userID, idempotencyKey uuid.UUID,
) (*readmodel.ReservationRM, error) {
	resourceEntity, err := r.validateAndGetResource(ctx, req.ResourceID)
	if err != nil {
		return nil, err
	}

	couponEntity, err := r.validateAndGetCoupon(ctx, req.GetCouponCode())
	if err != nil {
		return nil, err
	}

	reservationEntity, err := req.ToDomain(userID, resourceEntity, couponEntity)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidationFailed)
	}

	return r.executeReservationTransaction(ctx, reservationEntity, idempotencyKey, userID)
}

func (r *reservationUseCaseImpl) executeReservationTransaction(
	ctx context.Context,
	reservationEntity *reservation.Reservation,
	idempotencyKey, userID uuid.UUID,
) (*readmodel.ReservationRM, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}
	defer func() {
		if rollbackErr := tx.Rollback(ctx); rollbackErr != nil {
			slog.Warn("failed to rollback transaction", "error", rollbackErr)
		}
	}()

	reservationRM, err := r.reservationRepo.Create(ctx, tx, reservationEntity)
	if err != nil {
		if infra.IsKind(err, infra.KindConflict) {
			return nil, ErrReservationConflict
		}
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if notificationErr := r.createNotificationJob(ctx, tx, reservationRM); notificationErr != nil {
		return nil, errs.Mark(notificationErr, ErrDatabaseOperationFailed)
	}

	responseBodyHash := r.calculateResponseHash(reservationRM)
	err = r.idempotencyRepo.UpdateStatusCompleted(ctx, tx, idempotencyKey, userID, responseBodyHash, reservationRM.ID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	return reservationRM, nil
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
		return nil, errs.Wrap(err, "failed to find resource")
	}

	return resource.NewResource(resourceRM.ID, resourceRM.Name, int(resourceRM.LeadTimeMin))
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
		return nil, errs.Wrap(err, "failed to find coupon")
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
		return nil, errs.Wrap(err, "failed to create coupon")
	}

	if err := couponEntity.ValidateUsage(r.clock.Now()); err != nil {
		return nil, ErrInvalidCoupon
	}

	return couponEntity, nil
}

func (r *reservationUseCaseImpl) createNotificationJob(
	ctx context.Context,
	tx sqlc.DBTX,
	reservationRM *readmodel.ReservationRM,
) error {
	notificationPayload, err := json.Marshal(map[string]any{
		"reservation_id": reservationRM.ID,
		"user_email":     reservationRM.UserEmail,
		"resource_name":  reservationRM.ResourceName,
		"slot":           reservationRM.Slot,
	})
	if err != nil {
		return err
	}

	return r.notificationRepo.CreateJob(ctx, tx, "reservation_created", notificationPayload, r.clock.Now())
}

func (r *reservationUseCaseImpl) GetReservation(ctx context.Context, id uuid.UUID) (*readmodel.ReservationRM, error) {
	reservation, err := r.reservationRepo.FindByID(ctx, id)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrReservationNotFound
		}
		return nil, errs.Wrap(err, "failed to find reservation")
	}

	return reservation, nil
}

func (r *reservationUseCaseImpl) GetUserReservations(ctx context.Context, userID uuid.UUID) ([]*readmodel.ReservationListRM, error) {
	reservations, err := r.reservationRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, errs.Wrap(err, "failed to find user reservations")
	}

	return reservations, nil
}

func (r *reservationUseCaseImpl) calculateRequestHash(req reqdto.CreateReservationRequest) string {
	data, _ := json.Marshal(req)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func (r *reservationUseCaseImpl) calculateResponseHash(reservationRM *readmodel.ReservationRM) string {
	data, _ := json.Marshal(reservationRM)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
