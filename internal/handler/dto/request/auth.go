package request

import (
	"gin-clean-starter/internal/domain/user"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (r *LoginRequest) ToDomain() (user.Credentials, error) {
	return user.NewCredentials(r.Email, r.Password)
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}
