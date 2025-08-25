package middleware

import (
	"log/slog"
	"net/http"
	"strings"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/pkg/cookie"
	"gin-clean-starter/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthMiddleware struct {
	tokenValidator usecase.TokenValidator
}

const (
	ctxUserIDKey   = "user_id"
	ctxUserRoleKey = "user_role"
)

var roleHierarchy = map[user.Role]int{
	user.RoleViewer:   1,
	user.RoleOperator: 2,
	user.RoleAdmin:    3,
}

func NewAuthMiddleware(tokenValidator usecase.TokenValidator) *AuthMiddleware {
	return &AuthMiddleware{
		tokenValidator: tokenValidator,
	}
}

func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string

		token = cookie.GetAccessToken(c)

		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimSpace(authHeader[len("Bearer "):])
			}
		}

		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Access token required",
			})
			c.Abort()
			return
		}

		userID, role, err := m.tokenValidator.ValidateToken(token)
		if err != nil {
			slog.Warn("Token validation failed in auth middleware", "error", err.Error())
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set(ctxUserIDKey, userID)
		c.Set(ctxUserRoleKey, role)
		c.Set("jwt_claims", map[string]any{
			"user_id": userID.String(),
			"role":    string(role),
		})
		c.Next()
	}
}

func hasMinimumRole(userRole, minRole user.Role) bool {
	userLevel, userExists := roleHierarchy[userRole]
	minLevel, minExists := roleHierarchy[minRole]
	return userExists && minExists && userLevel >= minLevel
}

// FOR FUTURE USE
func (m *AuthMiddleware) RequireRoleAtLeast(minRole user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRole(c)
		if !ok {
			// Unexpected error: should be used after RequireAuth()
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Internal server error",
			})
			c.Abort()
			return
		}

		if !hasMinimumRole(role, minRole) {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "Insufficient permissions",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// FOR FUTURE USE: authenticates the request if a token is present, but does not abort on failure.
func (m *AuthMiddleware) OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		var token string
		token = cookie.GetAccessToken(c)
		if token == "" {
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimSpace(authHeader[len("Bearer "):])
			}
		}

		if token == "" {
			// No token present; continue without setting context.
			c.Next()
			return
		}

		userID, role, err := m.tokenValidator.ValidateToken(token)
		if err != nil {
			// Invalid token; continue without aborting.
			c.Next()
			return
		}

		c.Set(ctxUserIDKey, userID)
		c.Set(ctxUserRoleKey, role)
		c.Set("jwt_claims", map[string]any{
			"user_id": userID.String(),
			"role":    string(role),
		})
		c.Next()
	}
}

func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get(ctxUserIDKey)
	if !exists {
		return uuid.Nil, false
	}

	id, ok := userID.(uuid.UUID)
	return id, ok
}

// FOR FUTURE USE: GetUserRole returns the authenticated user role from context
func GetUserRole(c *gin.Context) (user.Role, bool) {
	userRole, exists := c.Get(ctxUserRoleKey)
	if !exists {
		return "", false
	}

	role, ok := userRole.(user.Role)
	return role, ok
}
