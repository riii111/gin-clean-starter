package errs

import "errors"

// Domain-specific sentinel errors for CQRS usecase layers
var (
	// Resource errors
	ErrResourceNotFound = errors.New("resource not found")

	// Reservation errors
	ErrReservationNotFound  = errors.New("reservation not found")
	ErrReservationConflict  = errors.New("reservation conflict")
	ErrDuplicateReservation = errors.New("duplicate reservation")
	ErrInvalidTimeSlot      = errors.New("invalid time slot")
	ErrInsufficientLeadTime = errors.New("insufficient lead time")

	// Coupon errors
	ErrCouponNotFound = errors.New("coupon not found")
	ErrInvalidCoupon  = errors.New("invalid coupon")

	// Idempotency errors
	ErrIdempotencyKeyRequired = errors.New("idempotency key required")
	ErrIdempotencyInProgress  = errors.New("idempotency in progress")
	ErrIdempotencyCheckFailed = errors.New("idempotency check failed")

	// Validation errors
	ErrDomainValidation       = errors.New("domain validation error")
	ErrDomainValidationFailed = errors.New("domain validation failed")

	// Operation errors
	ErrDatabaseOperationFailed = errors.New("database operation failed")
)
