package api

import (
	"errors"
	"log/slog"
	"net/http"

	reqdto "gin-clean-starter/internal/handler/dto/request"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/handler/httperr"
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
		slog.Warn("Invalid request format in login", "error", err)
		httperr.AbortWithError(c, http.StatusBadRequest, err,
			"Invalid request format", nil)
		return
	}

	credentials, err := req.ToDomain()
	if err != nil {
		slog.Warn("Invalid request data in login", "error", err)
		httperr.AbortWithError(c, http.StatusBadRequest, err,
			"Invalid request data", nil)
		return
	}

	pair, user, err := h.authUseCase.Login(c.Request.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrInvalidCredentials),
			errors.Is(err, usecase.ErrUserNotFound):
			slog.Warn("Login failed due to invalid credentials",
				"email", credentials.Email, "error", err)
			httperr.AbortWithError(c, http.StatusUnauthorized, err,
				"Invalid email or password", nil)
		case errors.Is(err, usecase.ErrUserInactive):
			slog.Warn("Login failed due to inactive user",
				"email", credentials.Email, "error", err)
			httperr.AbortWithError(c, http.StatusForbidden, err,
				"Account is inactive", nil)
		default:
			slog.Error("Unexpected error in login", "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err,
				"Internal server error", nil)
		}
		return
	}

	cookie.SetTokenCookies(c, h.cfg.Cookie, pair.AccessToken, pair.RefreshToken,
		h.jwtService.GetAccessTokenDuration(), h.jwtService.GetRefreshTokenDuration())

	slog.Info("User logged in successfully", "user_id", user.ID)
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
// @Success 200 {object} queries.AuthorizedUserView
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /auth/me [get]
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		slog.Error("User ID not found in context")
		httperr.AbortWithError(c, http.StatusInternalServerError,
			errors.New("user_id not found in context"),
			"Internal server error", nil)
		return
	}

	user, err := h.authUseCase.GetCurrentUser(c.Request.Context(), userID)
	if err != nil {
		switch {
		case errors.Is(err, usecase.ErrUserNotFound):
			slog.Warn("User not found", "user_id", userID, "error", err)
			httperr.AbortWithError(c, http.StatusNotFound, err,
				"User not found", nil)
		case errors.Is(err, usecase.ErrUserInactive):
			slog.Warn("User account is inactive", "user_id", userID, "error", err)
			httperr.AbortWithError(c, http.StatusForbidden, err,
				"Account is inactive", nil)
		default:
			slog.Error("Unexpected error in get current user", "user_id", userID, "error", err)
			httperr.AbortWithError(c, http.StatusInternalServerError, err,
				"Internal server error", nil)
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
		slog.Warn("Refresh token not found in cookie")
		httperr.AbortWithError(c, http.StatusUnauthorized,
			errors.New("refresh token not found in cookie"),
			"Refresh token not found", nil)
		return
	}

	pair, err := h.authUseCase.RefreshToken(c.Request.Context(), refreshToken)
	if err != nil {
		slog.Warn("Token refresh failed", "error", err)
		httperr.AbortWithError(c, http.StatusUnauthorized, err,
			"Invalid or expired refresh token", nil)
		return
	}

	cookie.SetTokenCookies(c, h.cfg.Cookie, pair.AccessToken, pair.RefreshToken,
		h.jwtService.GetAccessTokenDuration(), h.jwtService.GetRefreshTokenDuration())

	slog.Info("Token refreshed successfully")
	c.JSON(http.StatusOK, gin.H{
		"message": "Token refreshed successfully",
	})
}
