//go:build unit || e2e

package builder

import (
	reqdto "gin-clean-starter/internal/handler/dto/request"
)

type AuthBuilder struct {
	Email    string
	Password string
}

func NewAuthBuilder() *AuthBuilder {
	return &AuthBuilder{
		Email:    "test@example.com",
		Password: "password123",
	}
}

func (a *AuthBuilder) BuildDTO() reqdto.LoginRequest {
	return reqdto.LoginRequest{
		Email:    a.Email,
		Password: a.Password,
	}
}
