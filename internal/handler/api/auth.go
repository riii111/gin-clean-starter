package api

import (
	"errors"
	"net/http"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthHandler struct {
	authUseCase usecase.AuthUseCase
}

func NewAuthHandler(authUseCase usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{
		authUseCase: authUseCase,
	}
}

// @Summary User login
// @Description Login with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body reqdto.LoginRequest true "Login request"
// @Success 200 {object} resdto.LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req reqdto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	credentials, err := req.ToDomain()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request data",
		})
		return
	}

	token, user, err := h.authUseCase.Login(c.Request.Context(), credentials)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidCredentials):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid email or password",
			})
		case errors.Is(err, usecase.ErrUserNotFound):
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid email or password",
			})
		case errors.Is(err, usecase.ErrUserInactive):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Account is inactive",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
		}
		return
	}

	response := resdto.LoginResponse{
		AccessToken: token,
		User:        user,
	}
	c.JSON(http.StatusOK, response)
}

// @Summary User logout
// @Description Logout current user session
// @Tags auth
// @Security BearerAuth
// @Success 204 "No Content"
// @Failure 401 {object} map[string]string
// @Router /auth/logout [post]
func (h *AuthHandler) Logout(c *gin.Context) {
	// For JWT-based stateless authentication, logout is handled client-side
	// by removing the token from client storage. Server simply returns 204 No Content.
	c.Status(http.StatusNoContent)
}

// @Summary Get current user
// @Description Get current authenticated user information
// @Tags auth
// @Security BearerAuth
// @Produce json
// @Success 200 {object} readmodel.AuthorizedUserRM
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userIDStr, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "User not authenticated",
		})
		return
	}

	userID, ok := userIDStr.(uuid.UUID)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid user ID",
		})
		return
	}

	user, err := h.authUseCase.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUserNotFound):
			c.JSON(http.StatusNotFound, gin.H{
				"error": "User not found",
			})
		case errors.Is(err, usecase.ErrUserInactive):
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Account is inactive",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
		}
		return
	}

	c.JSON(http.StatusOK, user)
}
