package commands

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
	"gin-clean-starter/internal/usecase/queries"

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
	ErrIdempotencyInProgress  = errors.New("idempotency request in progress")
	ErrDomainValidation       = errors.New("domain validation failed")

	// Error markers for categorization
	ErrDomainValidationFailed  = errors.New("domain validation failed")
	ErrIdempotencyCheckFailed  = errors.New("idempotency check failed")
	ErrDatabaseOperationFailed = errors.New("database operation failed")
)

type CreateReservationResult struct {
	Reservation *queries.ReservationView
	IsReplayed  bool
}

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (*queries.ReservationView, error)
}

type ResourceRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*queries.ResourceView, error)
}

type CouponRepository interface {
	FindByCode(ctx context.Context, code string) (*queries.CouponView, error)
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	Get(ctx context.Context, key uuid.UUID, userID uuid.UUID) (*queries.IdempotencyKeyView, error)
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, responseBodyHash string, resultReservationID uuid.UUID) error
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error
}

type ReservationUseCase interface {
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
) ReservationUseCase {
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
			dummyActor := uuid.New() // TODO: pass actual actor if needed
			return r.reservationQueries.GetByID(ctx, dummyActor, *existing.ResultReservationID)
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

	reservationView, err := r.reservationRepo.Create(ctx, tx, reservationEntity)
	if err != nil {
		if infra.IsKind(err, infra.KindConflict) {
			return nil, ErrReservationConflict
		}
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if notificationErr := r.createNotificationJob(ctx, tx, reservationView); notificationErr != nil {
		return nil, errs.Mark(notificationErr, ErrDatabaseOperationFailed)
	}

	responseBodyHash := r.calculateResponseHash(reservationView)
	err = r.idempotencyRepo.UpdateStatusCompleted(ctx, tx, idempotencyKey, userID, responseBodyHash, reservationView.ID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if err := tx.Commit(ctx); err != nil {
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
	reservationView *queries.ReservationView,
) error {
	notificationPayload, err := json.Marshal(map[string]any{
		"reservation_id": reservationView.ID,
		"user_email":     reservationView.UserEmail,
		"resource_name":  reservationView.ResourceName,
		"slot":           reservationView.Slot,
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

func (r *reservationUseCaseImpl) calculateResponseHash(reservationView *queries.ReservationView) string {
	data, _ := json.Marshal(reservationView)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
