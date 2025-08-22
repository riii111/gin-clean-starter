package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/handler/httperr"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ReservationHandler struct {
	reservationUseCase usecase.ReservationUseCase
}

func NewReservationHandler(reservationUseCase usecase.ReservationUseCase) *ReservationHandler {
	return &ReservationHandler{
		reservationUseCase: reservationUseCase,
	}
}

// @Summary Create reservation
// @Description Create a new reservation with idempotency key
// @Tags reservations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param Idempotency-Key header string true "Idempotency key for duplicate prevention"
// @Param request body reqdto.CreateReservationRequest true "Reservation request"
// @Success 201 {object} resdto.ReservationResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Router /reservations [post]
func (h *ReservationHandler) CreateReservation(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		slog.Error("Failed to get user ID from context")
		httperr.AbortWithError(c, http.StatusInternalServerError,
			errors.New("user context missing"), httperr.TypeInternal,
			"Internal server error", nil)
		return
	}

	idempotencyKey, err := h.getIdempotencyKey(c)
	if err != nil {
		slog.Warn("Invalid idempotency key", "error", err)
		httperr.AbortWithError(c, http.StatusBadRequest, err, httperr.TypeBadRequest,
			err.Error(), nil)
		return
	}

	var req reqdto.CreateReservationRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		slog.Warn("Invalid request format in create reservation", "error", bindErr)
		httperr.AbortWithError(c, http.StatusBadRequest, bindErr, httperr.TypeValidation,
			"Invalid request format", nil)
		return
	}

	result, err := h.reservationUseCase.CreateReservation(c.Request.Context(), req, userID, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrResourceNotFound):
			slog.Warn("Resource not found", "resource_id", req.ResourceID, "error", err)
			httperr.AbortWithError(c, http.StatusNotFound, err, httperr.TypeNotFound,
				"Resource not found", nil)
		case errors.Is(err, usecase.ErrCouponNotFound):
			slog.Warn("Coupon not found", "coupon_code", req.GetCouponCode(), "error", err)
			httperr.AbortWithError(c, http.StatusNotFound, err, httperr.TypeNotFound,
				"Coupon not found", nil)
		case errors.Is(err, usecase.ErrInvalidTimeSlot):
			slog.Warn("Invalid time slot", "start_time", req.StartTime, "end_time", req.EndTime, "error", err)
			httperr.AbortWithError(c, http.StatusBadRequest, err, httperr.TypeBadRequest,
				"Invalid time slot", nil)
		case errors.Is(err, usecase.ErrInsufficientLeadTime):
			slog.Warn("Insufficient lead time", "start_time", req.StartTime, "error", err)
			httperr.AbortWithError(c, http.StatusBadRequest, err, httperr.TypeBadRequest,
				"Insufficient lead time for reservation", nil)
		case errors.Is(err, usecase.ErrInvalidCoupon):
			slog.Warn("Invalid or expired coupon", "coupon_code", req.GetCouponCode(), "error", err)
			httperr.AbortWithError(c, http.StatusBadRequest, err, httperr.TypeBadRequest,
				"Invalid or expired coupon", nil)
		case errors.Is(err, usecase.ErrDuplicateReservation):
			slog.Warn("Duplicate reservation request", "idempotency_key", idempotencyKey, "error", err)
			httperr.AbortWithError(c, http.StatusConflict, err, httperr.TypeConflict,
				"Duplicate reservation request with different parameters", nil)
		case errors.Is(err, usecase.ErrReservationConflict):
			slog.Warn("Time slot conflict", "resource_id", req.ResourceID, "start_time", req.StartTime, "error", err)
			httperr.AbortWithError(c, http.StatusConflict, err, httperr.TypeConflict,
				"Time slot is already reserved", nil)
		case errors.Is(err, usecase.ErrIdempotencyInProgress):
			slog.Info("Reservation request in progress", "idempotency_key", idempotencyKey)
			httperr.AbortWithError(c, http.StatusAccepted, err, httperr.TypeConflict,
				"Reservation request is currently being processed", map[string]string{"retry_after": "2"})
		case errors.Is(err, usecase.ErrDomainValidation):
			slog.Warn("Domain validation failed", "user_id", userID, "error", err)
			httperr.AbortWithError(c, http.StatusUnprocessableEntity, err, httperr.TypeValidation,
				"Domain validation failed", nil)
		default:
			slog.Error("Unexpected error in create reservation", "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err, httperr.TypeInternal,
				"Internal server error", nil)
		}
		return
	}

	response := resdto.FromReservationRM(result.ReservationRM)

	c.Header("Location", "/reservations/"+result.ReservationRM.ID.String())
	if result.IsReplayed {
		c.Header("Idempotent-Replayed", "true")
		c.JSON(http.StatusOK, response)
	} else {
		c.JSON(http.StatusCreated, response)
	}
}

// @Summary Get reservation
// @Description Get reservation by ID
// @Tags reservations
// @Produce json
// @Security BearerAuth
// @Param id path string true "Reservation ID"
// @Success 200 {object} resdto.ReservationResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /reservations/{id} [get]
func (h *ReservationHandler) GetReservation(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		slog.Warn("Invalid reservation ID format", "id", idStr, "error", err)
		httperr.AbortWithError(c, http.StatusBadRequest, err, httperr.TypeBadRequest,
			"Invalid reservation ID format", nil)
		return
	}

	reservationRM, err := h.reservationUseCase.GetReservation(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrReservationNotFound):
			slog.Info("Reservation not found", "id", id)
			httperr.AbortWithError(c, http.StatusNotFound, err, httperr.TypeNotFound,
				"Reservation not found", nil)
		default:
			slog.Error("Unexpected error in get reservation", "id", id, "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err, httperr.TypeInternal,
				"Internal server error", nil)
		}
		return
	}

	etag := fmt.Sprintf(`W/"%s-%d"`, reservationRM.ID, reservationRM.UpdatedAt.UnixNano())
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	c.Header("ETag", etag)
	response := resdto.FromReservationRM(reservationRM)
	c.JSON(http.StatusOK, response)
}

// @Summary Get user reservations
// @Description Get all reservations for the current user
// @Tags reservations
// @Produce json
// @Security BearerAuth
// @Success 200 {array} resdto.ReservationListResponse
// @Failure 401 {object} map[string]string
// @Router /reservations [get]
func (h *ReservationHandler) GetUserReservations(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		slog.Error("Failed to get user ID from context")
		httperr.AbortWithError(c, http.StatusInternalServerError,
			errors.New("user context missing"), httperr.TypeInternal,
			"Internal server error", nil)
		return
	}

	reservationsRM, err := h.reservationUseCase.GetUserReservations(c.Request.Context(), userID)
	if err != nil {
		slog.Error("Unexpected error in get user reservations", "user_id", userID, "error", err)
		httperr.AbortWithError(c, http.StatusInternalServerError, err, httperr.TypeInternal,
			"Internal server error", nil)
		return
	}

	response := make([]*resdto.ReservationListResponse, len(reservationsRM))
	for i, rm := range reservationsRM {
		response[i] = resdto.FromReservationListRM(rm)
	}

	c.JSON(http.StatusOK, response)
}

func (h *ReservationHandler) getIdempotencyKey(c *gin.Context) (uuid.UUID, error) {
	keyStr := c.GetHeader("Idempotency-Key")
	if keyStr == "" {
		return uuid.Nil, usecase.ErrIdempotencyKeyRequired
	}

	key, err := uuid.Parse(keyStr)
	if err != nil {
		return uuid.Nil, errors.New("invalid idempotency key format")
	}

	return key, nil
}
