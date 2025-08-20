package api

import (
	"errors"
	"net/http"
	"strings"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	idempotencyKey, err := h.getIdempotencyKey(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	var req reqdto.CreateReservationRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	params := usecase.CreateReservationParams{
		ResourceID: req.ResourceID,
		UserID:     userID,
		StartTime:  req.StartTime,
		EndTime:    req.EndTime,
		CouponCode: req.GetCouponCode(),
		Note:       req.Note,
	}

	reservationRM, err := h.reservationUseCase.CreateReservation(c.Request.Context(), params, idempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrResourceNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Resource not found",
			})
		case errors.Is(err, usecase.ErrCouponNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Coupon not found",
			})
		case errors.Is(err, usecase.ErrInvalidTimeSlot):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid time slot",
			})
		case errors.Is(err, usecase.ErrInsufficientLeadTime):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Insufficient lead time for reservation",
			})
		case errors.Is(err, usecase.ErrInvalidCoupon):
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid or expired coupon",
			})
		case errors.Is(err, usecase.ErrDuplicateReservation):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Duplicate reservation request with different parameters",
			})
		case strings.Contains(err.Error(), "reservation in progress"):
			c.JSON(http.StatusConflict, gin.H{
				"error": "Reservation request is currently being processed",
			})
		case strings.Contains(err.Error(), "domain validation failed"):
			c.JSON(http.StatusUnprocessableEntity, gin.H{
				"error": "Domain validation failed",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
		}
		return
	}

	response := resdto.FromReservationRM(reservationRM)
	c.JSON(http.StatusCreated, response)
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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid reservation ID format",
		})
		return
	}

	reservationRM, err := h.reservationUseCase.GetReservation(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrReservationNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Reservation not found",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
		}
		return
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
		return
	}

	reservationsRM, err := h.reservationUseCase.GetUserReservations(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
		})
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
