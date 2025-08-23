package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrResourceNotFound           = errs.New("resource not found")
	ErrCouponNotFound             = errs.New("coupon not found")
	ErrInvalidTimeSlot            = errs.New("invalid time slot")
	ErrInsufficientLeadTime       = errs.New("insufficient lead time")
	ErrDuplicateReservation       = errs.New("duplicate reservation")
	ErrReservationConflict        = errs.New("reservation conflict")
	ErrInvalidCoupon              = errs.New("invalid coupon")
	ErrIdempotencyInProgress      = errs.New("idempotency in progress")
	ErrDomainValidation           = errs.New("domain validation error")
	ErrIdempotencyCheckFailed     = errs.New("idempotency check failed")
	ErrDatabaseOperationFailed    = errs.New("database operation failed")
	ErrMissingResultReservationID = errs.New("completed request missing result reservation ID")
	ErrInvalidIdempotencyStatus   = errs.New("invalid idempotency key status")
)

type CreateReservationResult struct {
	Reservation *queries.ReservationView
	IsReplayed  bool
}

type ValidationResult struct {
	Resource *resource.Resource
	Coupon   *coupon.Coupon
	TimeSlot reservation.TimeSlot
	Note     reservation.Note
}

type ReservationRepository interface {
	Create(ctx context.Context, tx sqlc.DBTX, res *reservation.Reservation) (uuid.UUID, error)
}

type IdempotencyRepository interface {
	TryInsert(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, endpoint, requestHash string, expiresAt time.Time) error
	UpdateStatusCompleted(ctx context.Context, tx sqlc.DBTX, key uuid.UUID, userID uuid.UUID, responseBodyHash string, resultReservationID uuid.UUID) error
}

type NotificationRepository interface {
	CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error
}

type ResourceStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*shared.ResourceSnapshot, error)
}

type CouponStore interface {
	FindByCode(ctx context.Context, code string) (*shared.CouponSnapshot, error)
}

type IdempotencyStore interface {
	Get(ctx context.Context, tx sqlc.DBTX, key, userID uuid.UUID) (*shared.IdempotencyRecord, error)
}

type ReservationStore interface {
	FindByID(ctx context.Context, id uuid.UUID) (*queries.ReservationView, error)
}

type ReservationCommands interface {
	CreateReservation(ctx context.Context, req reqdto.CreateReservationRequest, userID uuid.UUID, idempotencyKey uuid.UUID) (*CreateReservationResult, error)
}

type reservationUseCaseImpl struct {
	reservationRepo    ReservationRepository
	resourceStore      ResourceStore
	couponStore        CouponStore
	idempotencyRepo    IdempotencyRepository
	idempotencyStore   IdempotencyStore
	notificationRepo   NotificationRepository
	reservationFactory *reservation.Factory
	reservationStore   ReservationStore
	db                 *pgxpool.Pool
	clock              clock.Clock
}

func NewReservationUseCase(
	reservationRepo ReservationRepository,
	resourceStore ResourceStore,
	couponStore CouponStore,
	idempotencyRepo IdempotencyRepository,
	idempotencyStore IdempotencyStore,
	notificationRepo NotificationRepository,
	reservationFactory *reservation.Factory,
	reservationStore ReservationStore,
	db *pgxpool.Pool,
	clock clock.Clock,
) ReservationCommands {
	return &reservationUseCaseImpl{
		reservationRepo:    reservationRepo,
		resourceStore:      resourceStore,
		couponStore:        couponStore,
		idempotencyRepo:    idempotencyRepo,
		idempotencyStore:   idempotencyStore,
		notificationRepo:   notificationRepo,
		reservationFactory: reservationFactory,
		reservationStore:   reservationStore,
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

	return shared.RunInTx(ctx, r.db, func(tx sqlc.DBTX) (*CreateReservationResult, error) {
		existingResult, err := r.handleIdempotencyInTx(ctx, tx, idempotencyKey, userID, requestHash, expiresAt)
		if err != nil {
			return nil, err
		}
		if existingResult != nil {
			return &CreateReservationResult{
				Reservation: existingResult,
				IsReplayed:  true,
			}, nil
		}

		reservationView, err := r.createNewReservationInTx(ctx, tx, req, userID, idempotencyKey)
		if err != nil {
			return nil, err
		}
		return &CreateReservationResult{
			Reservation: reservationView,
			IsReplayed:  false,
		}, nil
	})
}

func (r *reservationUseCaseImpl) handleIdempotencyInTx(
	ctx context.Context,
	tx sqlc.DBTX,
	idempotencyKey, userID uuid.UUID,
	requestHash string,
	expiresAt time.Time,
) (*queries.ReservationView, error) {
	if err := r.idempotencyRepo.TryInsert(ctx, tx, idempotencyKey, userID, "POST /reservations", requestHash, expiresAt); err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	existing, err := r.idempotencyStore.Get(ctx, tx, idempotencyKey, userID)
	if err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	switch existing.Status {
	case "completed":
		if existing.ResultReservationID != nil {
			reservation, err := r.reservationStore.FindByID(ctx, *existing.ResultReservationID)
			if err != nil {
				return nil, errs.Mark(err, ErrDatabaseOperationFailed)
			}
			return reservation, nil
		}
		return nil, ErrMissingResultReservationID

	case "processing":
		if existing.RequestHash != requestHash {
			return nil, ErrDuplicateReservation
		}
		return nil, ErrIdempotencyInProgress

	default:
		return nil, ErrInvalidIdempotencyStatus
	}
}

func (r *reservationUseCaseImpl) createNewReservationInTx(
	ctx context.Context,
	tx sqlc.DBTX,
	req reqdto.CreateReservationRequest,
	userID, idempotencyKey uuid.UUID,
) (*queries.ReservationView, error) {
	validationResult, err := r.validateInputs(ctx, req)
	if err != nil {
		return nil, err
	}

	reservationEntity, err := r.reservationFactory.CreateReservation(
		validationResult.Resource,
		userID,
		validationResult.TimeSlot,
		validationResult.Coupon,
		validationResult.Note,
	)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidation)
	}

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

	tempHash := r.calculateIDHash(reservationID)
	err = r.idempotencyRepo.UpdateStatusCompleted(ctx, tx, idempotencyKey, userID, tempHash, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	reservationView, err := r.reservationStore.FindByID(ctx, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	return reservationView, nil
}

func (r *reservationUseCaseImpl) validateInputs(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
) (*ValidationResult, error) {
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

	return &ValidationResult{
		Resource: resourceEntity,
		Coupon:   couponEntity,
		TimeSlot: domainData.TimeSlot,
		Note:     domainData.Note,
	}, nil
}

func (r *reservationUseCaseImpl) validateAndGetResource(
	ctx context.Context,
	resourceID uuid.UUID,
) (*resource.Resource, error) {
	resourceRM, err := r.resourceStore.FindByID(ctx, resourceID)
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

	couponRM, err := r.couponStore.FindByCode(ctx, *couponCode)
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
