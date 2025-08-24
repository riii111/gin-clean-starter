package commands

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
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

	NotificationKindEmail               = "email"
	NotificationTopicReservationCreated = "reservation_created"
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
	uow      shared.UnitOfWork
	services *reservation.Services
	clock    clock.Clock
}

func NewReservationUseCase(
	uow shared.UnitOfWork,
	services *reservation.Services,
	clock clock.Clock,
) ReservationCommands {
	return &reservationUseCaseImpl{
		uow:      uow,
		services: services,
		clock:    clock,
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

	requestHash := r.calculateNormalizedHash(req)
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
		reservationID, err = r.createReservation(ctx, tx, validationResult, userID, idempotencyKey)
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
	inserted := true
	if err := tx.Idempotency().TryInsert(ctx, tx.DB(), idempotencyKey, userID, EndpointCreateReservation, requestHash, expiresAt); err != nil {
		if !infra.IsKind(err, infra.KindConflict) {
			return nil, errs.Mark(err, errIdempotencyCheckFailed)
		}
		inserted = false
	}

	if !inserted {
		existing, err := tx.Reads().IdempotencyByKey(ctx, idempotencyKey, userID)
		if err != nil {
			return nil, errs.Mark(err, errIdempotencyCheckFailed)
		}

		// Treat expired keys as non-existent (allows new creation)
		if existing.ExpiresAt.Before(r.clock.Now()) {
			return nil, nil
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

	// New key inserted or existing key expired: proceed with creation
	return nil, nil
}

func (r *reservationUseCaseImpl) createReservation(
	ctx context.Context,
	tx shared.Tx,
	validationResult *ValidationResult,
	userID, idempotencyKey uuid.UUID,
) (*uuid.UUID, error) {
	reservationEntity, err := reservation.NewReservation(
		r.services,
		validationResult.Resource,
		userID,
		validationResult.TimeSlot,
		validationResult.Coupon,
		validationResult.Note,
	)
	if err != nil {
		if errors.Is(err, reservation.ErrLeadTimeNotMet) {
			return nil, ErrInsufficientLeadTime
		}
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
		return nil, errs.Mark(err, errDatabaseOperationFailed)
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
			return nil, errs.Mark(err, errDatabaseOperationFailed)
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
		"type":           NotificationTopicReservationCreated,
	})
	if err != nil {
		return err
	}

	return tx.Notifications().CreateJob(ctx, tx.DB(), NotificationKindEmail, NotificationTopicReservationCreated, notificationPayload, r.clock.Now())
}

func (r *reservationUseCaseImpl) calculateIDHash(id uuid.UUID) string {
	hash := sha256.Sum256([]byte(id.String()))
	return hex.EncodeToString(hash[:])
}

func (r *reservationUseCaseImpl) calculateNormalizedHash(req reqdto.CreateReservationRequest) string {
	normalizedCouponCode := req.GetCouponCode()
	if normalizedCouponCode != nil {
		lowered := strings.ToLower(*normalizedCouponCode)
		normalizedCouponCode = &lowered
	}

	normalized := reqdto.CreateReservationRequest{
		ResourceID: req.ResourceID,
		StartTime:  req.StartTime.UTC(),
		EndTime:    req.EndTime.UTC(),
		CouponCode: normalizedCouponCode,
		Note:       normalizeNote(req.Note),
	}
	data, _ := json.Marshal(normalized)
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func normalizeNote(note *string) *string {
	if note == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*note)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
