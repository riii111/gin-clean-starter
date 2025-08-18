package middleware

import (
	"net/http"
	"strings"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/pkg/cookie"
	"gin-clean-starter/internal/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AuthMiddleware struct {
	authUseCase usecase.AuthUseCase
}

const (
	ctxUserIDKey   = "user_id"
	ctxUserRoleKey = "user_role"
)

func NewAuthMiddleware(authUseCase usecase.AuthUseCase) *AuthMiddleware {
	return &AuthMiddleware{
		authUseCase: authUseCase,
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

		userID, role, err := m.authUseCase.ValidateToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "Invalid or expired token",
			})
			c.Abort()
			return
		}

		c.Set(ctxUserIDKey, userID)
		c.Set(ctxUserRoleKey, role)
		c.Next()
	}
}

func (m *AuthMiddleware) RequireRoleAtLeast(minRole user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, ok := GetUserRole(c)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error": "User role not found in context",
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

func hasMinimumRole(userRole, minRole user.Role) bool {
	roleHierarchy := map[user.Role]int{
		user.RoleViewer:   1,
		user.RoleOperator: 2,
		user.RoleAdmin:    3,
	}

	userLevel, userExists := roleHierarchy[userRole]
	minLevel, minExists := roleHierarchy[minRole]

	return userExists && minExists && userLevel >= minLevel
}

// GetUserID returns the authenticated user ID from context.
func GetUserID(c *gin.Context) (uuid.UUID, bool) {
	userID, exists := c.Get(ctxUserIDKey)
	if !exists {
		return uuid.Nil, false
	}

	id, ok := userID.(uuid.UUID)
	return id, ok
}

// GetUserRole returns the authenticated user role from context.
func GetUserRole(c *gin.Context) (user.Role, bool) {
	userRole, exists := c.Get(ctxUserRoleKey)
	if !exists {
		return "", false
	}

	role, ok := userRole.(user.Role)
	return role, ok
}
