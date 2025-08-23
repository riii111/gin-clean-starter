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
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/internal/usecase/shared"

	"github.com/google/uuid"
	"github.com/jinzhu/copier"
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
	requestHash := r.calculateRequestHash(req)
	expiresAt := r.clock.Now().Add(24 * time.Hour)

	var result *CreateReservationResult

	err := r.uow.Within(ctx, func(ctx context.Context, tx shared.Tx) error {
		existingResult, err := r.handleIdempotencyInTx(ctx, tx, idempotencyKey, userID, requestHash, expiresAt)
		if err != nil {
			return err
		}
		if existingResult != nil {
			result = &CreateReservationResult{
				Reservation: existingResult,
				IsReplayed:  true,
			}
			return nil
		}

		reservationView, err := r.createNewReservationInTx(ctx, tx, req, userID, idempotencyKey)
		if err != nil {
			return err
		}
		result = &CreateReservationResult{
			Reservation: reservationView,
			IsReplayed:  false,
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
) (*queries.ReservationView, error) {
	if err := tx.Idempotency().TryInsert(ctx, tx.DB(), idempotencyKey, userID, "POST /reservations", requestHash, expiresAt); err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	existing, err := tx.Reads().IdempotencyByKey(ctx, idempotencyKey, userID)
	if err != nil {
		return nil, errs.Mark(err, ErrIdempotencyCheckFailed)
	}

	switch existing.Status {
	case "completed":
		if existing.ResultReservationID != nil {
			reservation, err := tx.Reads().ReservationByID(ctx, *existing.ResultReservationID)
			if err != nil {
				return nil, errs.Mark(err, ErrDatabaseOperationFailed)
			}
			var reservationView queries.ReservationView
			if err := copier.Copy(&reservationView, reservation); err != nil {
				return nil, errs.Mark(err, ErrDatabaseOperationFailed)
			}
			return &reservationView, nil
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
	tx shared.Tx,
	req reqdto.CreateReservationRequest,
	userID, idempotencyKey uuid.UUID,
) (*queries.ReservationView, error) {
	validationResult, err := r.validateInputsInTx(ctx, tx, req)
	if err != nil {
		return nil, err
	}

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
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	if notificationErr := r.createNotificationJobByID(ctx, tx, reservationID); notificationErr != nil {
		return nil, errs.Mark(notificationErr, ErrDatabaseOperationFailed)
	}

	tempHash := r.calculateIDHash(reservationID)
	err = tx.Idempotency().UpdateStatusCompleted(ctx, tx.DB(), idempotencyKey, userID, tempHash, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	reservation, err := tx.Reads().ReservationByID(ctx, reservationID)
	if err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	var reservationView queries.ReservationView
	if err := copier.Copy(&reservationView, reservation); err != nil {
		return nil, errs.Mark(err, ErrDatabaseOperationFailed)
	}

	return &reservationView, nil
}

func (r *reservationUseCaseImpl) validateInputsInTx(
	ctx context.Context,
	tx shared.Tx,
	req reqdto.CreateReservationRequest,
) (*ValidationResult, error) {
	resourceEntity, err := r.validateAndGetResourceInTx(ctx, tx, req.ResourceID)
	if err != nil {
		return nil, err
	}

	couponEntity, err := r.validateAndGetCouponInTx(ctx, tx, req.GetCouponCode())
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

func (r *reservationUseCaseImpl) validateAndGetResourceInTx(
	ctx context.Context,
	tx shared.Tx,
	resourceID uuid.UUID,
) (*resource.Resource, error) {
	resourceRM, err := tx.Reads().ResourceByID(ctx, resourceID)
	if err != nil {
		if infra.IsKind(err, infra.KindNotFound) {
			return nil, ErrResourceNotFound
		}
		return nil, errs.Mark(err, ErrResourceNotFound)
	}

	return resource.NewResource(resourceRM.ID, resourceRM.Name, resourceRM.LeadTimeMin)
}

func (r *reservationUseCaseImpl) validateAndGetCouponInTx(
	ctx context.Context,
	tx shared.Tx,
	couponCode *string,
) (*coupon.Coupon, error) {
	if couponCode == nil {
		return nil, nil
	}

	couponRM, err := tx.Reads().CouponByCode(ctx, *couponCode)
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
