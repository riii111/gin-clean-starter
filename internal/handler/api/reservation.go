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
	"gin-clean-starter/internal/pkg/errs"
	"gin-clean-starter/internal/usecase/commands"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

var (
	ErrMissingUserContext          = errs.New("user context missing")
	ErrIdempotencyKeyRequired      = errs.New("idempotency key required")
	ErrInvalidIdempotencyKeyFormat = errs.New("invalid idempotency key format")
	ErrInvalidReservationIDFormat  = errs.New("invalid reservation ID format")
)

type ReservationHandler struct {
	reservationCommands commands.ReservationCommands
	reservationQueries  queries.ReservationQueries
}

func NewReservationHandler(reservationCommands commands.ReservationCommands, reservationQueries queries.ReservationQueries) *ReservationHandler {
	return &ReservationHandler{
		reservationCommands: reservationCommands,
		reservationQueries:  reservationQueries,
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
			ErrMissingUserContext,
			"Internal server error", nil)
		return
	}

	idempotencyKey, err := h.getIdempotencyKey(c)
	if err != nil {
		slog.Warn("Invalid idempotency key", "error", err)
		httperr.AbortWithError(c, http.StatusBadRequest, err,
			err.Error(), nil)
		return
	}

	var req reqdto.CreateReservationRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		slog.Warn("Invalid request format in create reservation", "error", bindErr)
		httperr.AbortWithError(c, http.StatusBadRequest, bindErr,
			"Invalid request format", nil)
		return
	}

	result, err := h.reservationCommands.CreateReservation(c.Request.Context(), req, userID, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, commands.ErrResourceNotFound),
			errors.Is(err, commands.ErrCouponNotFound):
			slog.Warn("Resource not found", "error", err)
			httperr.AbortWithError(c, http.StatusNotFound, err,
				"Resource not found", nil)
		case errors.Is(err, commands.ErrInvalidTimeSlot),
			errors.Is(err, commands.ErrInsufficientLeadTime),
			errors.Is(err, commands.ErrInvalidCoupon),
			errors.Is(err, commands.ErrDomainValidation):
			slog.Warn("Bad request", "error", err)
			httperr.AbortWithError(c, http.StatusBadRequest, err,
				"Invalid request parameters", nil)
		case errors.Is(err, commands.ErrDuplicateReservation),
			errors.Is(err, commands.ErrReservationConflict):
			slog.Warn("Conflict", "error", err)
			httperr.AbortWithError(c, http.StatusConflict, err,
				"Reservation conflict", nil)
		case errors.Is(err, commands.ErrIdempotencyInProgress):
			slog.Info("Reservation request in progress", "idempotency_key", idempotencyKey)
			httperr.AbortWithError(c, http.StatusAccepted, err,
				"Reservation request is currently being processed", map[string]string{"retry_after": "2"})
		default:
			slog.Error("Unexpected error in create reservation", "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err,
				"Internal server error", nil)
		}
		return
	}

	response := resdto.FromReservationView(result.Reservation)

	c.Header("Location", "/reservations/"+result.Reservation.ID.String())
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
		httperr.AbortWithError(c, http.StatusBadRequest, ErrInvalidReservationIDFormat,
			"Invalid reservation ID format", nil)
		return
	}

	// actor is the current user (authorization can be applied inside queries)
	actorID, _ := middleware.GetUserID(c)
	reservationRM, err := h.reservationQueries.GetByID(c.Request.Context(), actorID, id)
	if err != nil {
		switch {
		case errors.Is(err, queries.ErrReservationNotFound):
			slog.Warn("Reservation not found", "error", err)
			httperr.AbortWithError(c, http.StatusNotFound, err,
				"Reservation not found", nil)
		default:
			slog.Error("Unexpected error in get reservation", "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err,
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
	response := resdto.FromReservationView(reservationRM)
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
			ErrMissingUserContext,
			"Internal server error", nil)
		return
	}

	reservationsRM, _, err := h.reservationQueries.ListByUser(c.Request.Context(), userID, nil, 50)
	if err != nil {
		slog.Error("Unexpected error in get user reservations", "user_id", userID, "error", err)
		httperr.AbortWithError(c, http.StatusInternalServerError, err,
			"Internal server error", nil)
		return
	}

	response := make([]*resdto.ReservationListResponse, len(reservationsRM))
	for i, rm := range reservationsRM {
		response[i] = resdto.FromReservationListItem(rm)
	}

	c.JSON(http.StatusOK, response)
}

func (h *ReservationHandler) getIdempotencyKey(c *gin.Context) (uuid.UUID, error) {
	keyStr := c.GetHeader("Idempotency-Key")
	if keyStr == "" {
		return uuid.Nil, ErrIdempotencyKeyRequired
	}

	key, err := uuid.Parse(keyStr)
	if err != nil {
		return uuid.Nil, ErrInvalidIdempotencyKeyFormat
	}

	return key, nil
}
