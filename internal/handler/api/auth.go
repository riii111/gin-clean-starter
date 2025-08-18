package api

import (
	"errors"
	"net/http"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/handler/middleware"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/cookie"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/usecase"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authUseCase usecase.AuthUseCase
	jwtService  *jwt.Service
	cfg         config.Config
}

func NewAuthHandler(authUseCase usecase.AuthUseCase, jwtService *jwt.Service, cfg config.Config) *AuthHandler {
	return &AuthHandler{
		authUseCase: authUseCase,
		jwtService:  jwtService,
		cfg:         cfg,
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

	pair, user, err := h.authUseCase.Login(c.Request.Context(), credentials)
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

	cookie.SetTokenCookies(c, h.cfg.Cookie, pair.AccessToken, pair.RefreshToken,
		h.jwtService.GetAccessTokenDuration(), h.jwtService.GetRefreshTokenDuration())

	response := resdto.LoginResponse{User: user}
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
	cookie.ClearTokenCookies(c, h.cfg.Cookie)
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
	userID, ok := middleware.GetUserID(c)
	if !ok {
		// Unexpected error: auth middleware should guarantee user_id exists
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Internal server error",
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

// @Summary Refresh access token
// @Description Refresh access token using refresh token from cookie
// @Tags auth
// @Produce json
// @Success 200 {object} gin.H
// @Failure 401 {object} map[string]string
// @Router /auth/refresh [post]
func (h *AuthHandler) Refresh(c *gin.Context) {
	refreshToken := cookie.GetRefreshToken(c)
	if refreshToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Refresh token not found",
		})
		return
	}

	pair, err := h.authUseCase.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": "Invalid or expired refresh token",
		})
		return
	}

	cookie.SetTokenCookies(c, h.cfg.Cookie, pair.AccessToken, pair.RefreshToken,
		h.jwtService.GetAccessTokenDuration(), h.jwtService.GetRefreshTokenDuration())

	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
	})
}
