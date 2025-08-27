package api

import (
	"log/slog"
	"net/http"
	"strconv"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/handler/httperr"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/usecase/commands"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ReviewHandler struct {
	cmds commands.ReviewCommands
	q    queries.ReviewQueries
}

func NewReviewHandler(cmds commands.ReviewCommands, q queries.ReviewQueries) *ReviewHandler {
	return &ReviewHandler{cmds: cmds, q: q}
}

// @Summary Create review
// @Description Create a new review for a completed reservation
// @Tags reviews
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body reqdto.CreateReviewRequest true "Create review request"
// @Success 201 {object} resdto.ReviewResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /reviews [post]
func (h *ReviewHandler) Create(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		httperr.AbortWithError(c, http.StatusUnauthorized, nil, "Unauthorized", nil)
		return
	}
	var req reqdto.CreateReviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid request", nil)
		return
	}
	result, err := h.cmds.Create(c.Request.Context(), req, userID)
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Create review failed", nil)
		return
	}
	view, err := h.q.GetByID(c.Request.Context(), result.ReviewID)
	if err != nil {
		httperr.AbortWithError(c, http.StatusInternalServerError, err, "Failed to load review", nil)
		return
	}
	c.JSON(http.StatusCreated, resdto.FromReviewView(view))
}

// @Summary Get review
// @Description Get a review by ID
// @Tags reviews
// @Produce json
// @Param id path string true "Review ID"
// @Success 200 {object} resdto.ReviewResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /reviews/{id} [get]
func (h *ReviewHandler) Get(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid id", nil)
		return
	}
	view, err := h.q.GetByID(c.Request.Context(), id)
	if err != nil {
		httperr.AbortWithError(c, http.StatusNotFound, err, "Not found", nil)
		return
	}
	c.JSON(http.StatusOK, resdto.FromReviewView(view))
}

// @Summary Update review
// @Description Update own review by ID
// @Tags reviews
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Review ID"
// @Param request body reqdto.UpdateReviewRequest true "Update review request"
// @Success 200 {object} resdto.ReviewResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /reviews/{id} [put]
func (h *ReviewHandler) Update(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid id", nil)
		return
	}
	actorID, ok := middleware.GetUserID(c)
	if !ok {
		httperr.AbortWithError(c, http.StatusUnauthorized, nil, "Unauthorized", nil)
		return
	}
	var req reqdto.UpdateReviewRequest
	if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, bindErr, "Invalid request", nil)
		return
	}
	if err = h.cmds.Update(c.Request.Context(), id, req, actorID); err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Update failed", nil)
		return
	}
	view, err := h.q.GetByID(c.Request.Context(), id)
	if err != nil {
		httperr.AbortWithError(c, http.StatusInternalServerError, err, "Failed to load review", nil)
		return
	}
	c.JSON(http.StatusOK, resdto.FromReviewView(view))
}

// @Summary Delete review
// @Description Delete own review (admins can delete any)
// @Tags reviews
// @Security BearerAuth
// @Param id path string true "Review ID"
// @Success 204 "No Content"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /reviews/{id} [delete]
func (h *ReviewHandler) Delete(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid id", nil)
		return
	}
	actorID, ok := middleware.GetUserID(c)
	if !ok {
		httperr.AbortWithError(c, http.StatusUnauthorized, nil, "Unauthorized", nil)
		return
	}
	role, _ := middleware.GetUserRole(c)
	if err := h.cmds.Delete(c.Request.Context(), id, actorID, string(role)); err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Delete failed", nil)
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary List resource reviews
// @Description List reviews for a resource with optional rating filters and keyset pagination
// @Tags reviews
// @Produce json
// @Param id path string true "Resource ID"
// @Param min_rating query int false "Minimum rating (1-5)"
// @Param max_rating query int false "Maximum rating (1-5)"
// @Param limit query int false "Max items (default 20)"
// @Param after query string false "Cursor for keyset pagination"
// @Success 200 {array} resdto.ReviewListItemResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /resources/{id}/reviews [get]
func (h *ReviewHandler) ListByResource(c *gin.Context) {
	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid resource id", nil)
		return
	}
	var minPtr, maxPtr *int
	if v := c.Query("min_rating"); v != "" {
		if iv, e := strconv.Atoi(v); e == nil {
			minPtr = &iv
		}
	}
	if v := c.Query("max_rating"); v != "" {
		if iv, e := strconv.Atoi(v); e == nil {
			maxPtr = &iv
		}
	}
	limit := 20
	if v := c.Query("limit"); v != "" {
		if iv, e := strconv.Atoi(v); e == nil {
			limit = queries.ValidateLimit(iv)
		}
	}
	var cursor *queries.Cursor
	if after := c.Query("after"); after != "" {
		cursor = &queries.Cursor{After: after}
	}
	items, next, err := h.q.ListByResource(c.Request.Context(), resourceID, queries.ReviewFilters{MinRating: minPtr, MaxRating: maxPtr}, cursor, limit)
	if err != nil {
		slog.Error("list reviews by resource failed", "error", err)
		httperr.AbortWithError(c, http.StatusInternalServerError, err, "Internal error", nil)
		return
	}
	resp := gin.H{"reviews": resdto.FromReviewList(items)}
	if next != nil {
		resp["next_cursor"] = next.After
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary List user reviews
// @Description List reviews posted by a user (viewer can only access own)
// @Tags reviews
// @Produce json
// @Security BearerAuth
// @Param id path string true "User ID"
// @Param limit query int false "Max items (default 20)"
// @Param after query string false "Cursor for keyset pagination"
// @Success 200 {array} resdto.ReviewListItemResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /users/{id}/reviews [get]
func (h *ReviewHandler) ListByUser(c *gin.Context) {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid user id", nil)
		return
	}
	actorID, _ := middleware.GetUserID(c)
	role, _ := middleware.GetUserRole(c)
	limit := 20
	if v := c.Query("limit"); v != "" {
		if iv, e := strconv.Atoi(v); e == nil {
			limit = queries.ValidateLimit(iv)
		}
	}
	var cursor *queries.Cursor
	if after := c.Query("after"); after != "" {
		cursor = &queries.Cursor{After: after}
	}
	items, next, err := h.q.ListByUser(c.Request.Context(), userID, actorID, string(role), cursor, limit)
	if err != nil {
		httperr.AbortWithError(c, http.StatusForbidden, err, "Access denied", nil)
		return
	}
	resp := gin.H{"reviews": resdto.FromReviewList(items)}
	if next != nil {
		resp["next_cursor"] = next.After
	}
	c.JSON(http.StatusOK, resp)
}

// @Summary Resource rating stats
// @Description Get rating statistics for a resource
// @Tags reviews
// @Produce json
// @Param id path string true "Resource ID"
// @Success 200 {object} resdto.ResourceRatingStatsResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /resources/{id}/rating-stats [get]
func (h *ReviewHandler) ResourceRatingStats(c *gin.Context) {
	resourceID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		httperr.AbortWithError(c, http.StatusBadRequest, err, "Invalid resource id", nil)
		return
	}
	stats, err := h.q.GetResourceRatingStats(c.Request.Context(), resourceID)
	if err != nil {
		httperr.AbortWithError(c, http.StatusInternalServerError, err, "Failed to get stats", nil)
		return
	}
	c.JSON(http.StatusOK, resdto.FromResourceRatingStats(stats))
}
