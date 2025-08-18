package request

import (
	"gin-clean-starter/internal/domain/auth"
)

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

func (r *LoginRequest) ToDomain() (auth.Credentials, error) {
	return auth.NewCredentials(r.Email, r.Password)
}
