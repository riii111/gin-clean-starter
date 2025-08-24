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
	"gin-clean-starter/internal/pkg/clock"
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
)

// Constants for idempotency
const (
	EndpointCreateReservation = "POST /reservations"
	IdemStatusProcessing      = "processing"
	IdemStatusCompleted       = "completed"
)

// Public errors - used by handlers
var (
	ErrResourceNotFound      = errs.New("resource not found")
	ErrCouponNotFound        = errs.New("coupon not found")
	ErrInvalidTimeSlot       = errs.New("invalid time slot")
	ErrInsufficientLeadTime  = errs.New("insufficient lead time")
	ErrDuplicateReservation  = errs.New("duplicate reservation")
	ErrReservationConflict   = errs.New("reservation conflict")
	ErrInvalidCoupon         = errs.New("invalid coupon")
	ErrIdempotencyInProgress = errs.New("idempotency in progress")
	ErrDomainValidation      = errs.New("domain validation error")
)

// Private errors - internal use only
var (
	errIdempotencyCheckFailed     = errs.New("idempotency check failed")
	errDatabaseOperationFailed    = errs.New("database operation failed")
	errMissingResultReservationID = errs.New("completed request missing result reservation ID")
	errInvalidIdempotencyStatus   = errs.New("invalid idempotency key status")
)

type CreateReservationResult struct {
	ReservationID uuid.UUID
	IsReplayed    bool
}

type ValidationResult struct {
	Resource *resource.Resource
	Coupon   *coupon.Coupon
	TimeSlot reservation.TimeSlot
	Note     reservation.Note
}

type ReservationCommands interface {
	CreateReservation(ctx context.Context, req reqdto.CreateReservationRequest, userID uuid.UUID, idempotencyKey uuid.UUID) (*CreateReservationResult, error)
}

type reservationUseCaseImpl struct {
	uow     shared.UnitOfWork
	factory *reservation.Factory
	clock   clock.Clock
}

func NewReservationUseCase(
	uow shared.UnitOfWork,
	factory *reservation.Factory,
	clock clock.Clock,
) ReservationCommands {
	return &reservationUseCaseImpl{
		uow:     uow,
		factory: factory,
		clock:   clock,
	}
}

func (r *reservationUseCaseImpl) CreateReservation(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	userID uuid.UUID,
	idempotencyKey uuid.UUID,
) (*CreateReservationResult, error) {
	domainData, err := req.ToDomain()
	if err != nil {
		return nil, errs.Mark(err, ErrInvalidTimeSlot)
	}

	validationResult, err := r.validateResourceAndCoupon(ctx, req, domainData)
	if err != nil {
		return nil, err
	}

	requestHash := r.calculateRequestHash(req)
	expiresAt := r.clock.Now().Add(24 * time.Hour)

	var result *CreateReservationResult

	err = r.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		var existingReservationID *uuid.UUID
		existingReservationID, err = r.handleIdempotencyInTx(ctx, tx, idempotencyKey, userID, requestHash, expiresAt)
		if err != nil {
			return err
		}
		if existingReservationID != nil {
			result = &CreateReservationResult{
				ReservationID: *existingReservationID,
				IsReplayed:    true,
			}
			return nil
		}

		var reservationID *uuid.UUID
		reservationID, err = r.createNewReservationWithValidatedData(ctx, tx, validationResult, userID, idempotencyKey)
		if err != nil {
			return err
		}
		result = &CreateReservationResult{
			ReservationID: *reservationID,
			IsReplayed:    false,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *reservationUseCaseImpl) handleIdempotencyInTx(
	ctx context.Context,
	tx shared.Tx,
	idempotencyKey, userID uuid.UUID,
	requestHash string,
	expiresAt time.Time,
) (*uuid.UUID, error) {
	if err := tx.Idempotency().TryInsert(ctx, tx.DB(), idempotencyKey, userID, EndpointCreateReservation, requestHash, expiresAt); err != nil {
		return nil, errs.Mark(err, errIdempotencyCheckFailed)
	}

	existing, err := tx.Reads().IdempotencyByKey(ctx, idempotencyKey, userID)
	if err != nil {
		return nil, errs.Mark(err, errIdempotencyCheckFailed)
	}

	switch existing.Status {
	case IdemStatusCompleted:
		if existing.ResultReservationID != nil {
			return existing.ResultReservationID, nil
		}
		return nil, errMissingResultReservationID

	case IdemStatusProcessing:
		if existing.RequestHash != requestHash {
			return nil, ErrDuplicateReservation
		}
		return nil, ErrIdempotencyInProgress

	default:
		return nil, errInvalidIdempotencyStatus
	}
}

func (r *reservationUseCaseImpl) createNewReservationWithValidatedData(
	ctx context.Context,
	tx shared.Tx,
	validationResult *ValidationResult,
	userID, idempotencyKey uuid.UUID,
) (*uuid.UUID, error) {
	reservationEntity, err := r.factory.CreateReservation(
		validationResult.Resource,
		userID,
		validationResult.TimeSlot,
		validationResult.Coupon,
		validationResult.Note,
	)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidation)
	}

	reservationID, err := tx.Reservations().Create(ctx, tx.DB(), reservationEntity)
	if err != nil {
		if infra.IsKind(err, infra.KindConflict) {
			return nil, ErrReservationConflict
		}
		return nil, errs.Mark(err, errDatabaseOperationFailed)
	}

	if notificationErr := r.createNotificationJobByID(ctx, tx, reservationID); notificationErr != nil {
		return nil, errs.Mark(notificationErr, errDatabaseOperationFailed)
	}

	tempHash := r.calculateIDHash(reservationID)
	err = tx.Idempotency().UpdateStatusCompleted(ctx, tx.DB(), idempotencyKey, userID, tempHash, reservationID)
	if err != nil {
		return nil, errs.Mark(err, errDatabaseOperationFailed)
	}

	return &reservationID, nil
}

func (r *reservationUseCaseImpl) validateResourceAndCoupon(
	ctx context.Context,
	req reqdto.CreateReservationRequest,
	domainData *reqdto.DomainConversion,
) (*ValidationResult, error) {
	reads := r.uow.CommandReads()

	resourceRM, err := reads.ResourceByID(ctx, req.ResourceID)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, errs.Mark(err, ErrResourceNotFound)
	}

	resourceEntity, err := resource.NewResource(resourceRM.ID, resourceRM.Name, resourceRM.LeadTimeMin)
	if err != nil {
		return nil, errs.Mark(err, ErrDomainValidation)
	}

	// Coupon validation (if provided)
	var couponEntity *coupon.Coupon
	if couponCode := req.GetCouponCode(); couponCode != nil {
		couponRM, err := reads.CouponByCode(ctx, *couponCode)
		if err != nil {
			if infra.IsKind(err, infra.KindNotFound) {
				return nil, ErrCouponNotFound
			}
			return nil, errs.Mark(err, ErrCouponNotFound)
		}

		couponEntity, err = coupon.NewCoupon(
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
	}

	return &ValidationResult{
		Resource: resourceEntity,
		Coupon:   couponEntity,
		TimeSlot: domainData.TimeSlot,
		Note:     domainData.Note,
	}, nil
}

func (r *reservationUseCaseImpl) createNotificationJobByID(
	ctx context.Context,
	tx shared.Tx,
	reservationID uuid.UUID,
) error {
	notificationPayload, err := json.Marshal(map[string]any{
		"reservation_id": reservationID,
		"type":           "reservation_created",
	})
	if err != nil {
		return err
	}

	return tx.Notifications().CreateJob(ctx, tx.DB(), "email", "reservation_created", notificationPayload, r.clock.Now())
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
