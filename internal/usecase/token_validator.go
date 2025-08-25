package usecase

import (
	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/pkg/jwt"

	"github.com/google/uuid"
)

// TokenValidator provides token validation for middleware
type TokenValidator interface {
	ValidateToken(tokenString string) (uuid.UUID, user.Role, error)
}

type tokenValidatorImpl struct {
	jwtService *jwt.Service
}

func NewTokenValidator(jwtService *jwt.Service) TokenValidator {
	return &tokenValidatorImpl{
		jwtService: jwtService,
	}
}

func (t *tokenValidatorImpl) ValidateToken(tokenString string) (uuid.UUID, user.Role, error) {
	claims, err := t.jwtService.ValidateToken(tokenString)
	if err != nil {
		return uuid.Nil, "", err
	}

	role, err := user.NewRole(claims.Role)
	if err != nil {
		return uuid.Nil, "", err
	}

	return claims.UserID, role, nil
}
