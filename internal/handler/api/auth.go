package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct{}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{}
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
}

type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
}

// @Summary User login
// @Description Login with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body LoginRequest true "Login request"
// @Success 200 {object} LoginResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request format",
		})
		return
	}

	// TODO: 実際の認証ロジックを実装
	// 仮実装として固定値を返す
	if req.Email == "test@example.com" && req.Password == "password" {
		response := LoginResponse{
			Token: "dummy-jwt-token-12345",
			User: User{
				ID:    "user-123",
				Email: req.Email,
				Name:  "Test User",
			},
		}
		c.JSON(http.StatusOK, response)
		return
	}

	c.JSON(http.StatusUnauthorized, gin.H{
		"error": "Invalid email or password",
	})
}
