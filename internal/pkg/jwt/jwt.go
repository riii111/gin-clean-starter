package jwt

import (
	"errors"
	"time"

	"gin-clean-starter/internal/domain/user"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type TokenType string

const (
	TokenTypeAccess  TokenType = "access"
	TokenTypeRefresh TokenType = "refresh"
)

type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Role      string    `json:"role"`
	TokenType TokenType `json:"token_type"`
	jwt.RegisteredClaims
}

type Service struct {
	secretKey            []byte
	accessTokenDuration  time.Duration
	refreshTokenDuration time.Duration
	issuer               string
	audience             string
	keyID                string
}

func NewService(secretKey string, accessTokenDuration, refreshTokenDuration time.Duration) *Service {
	return &Service{
		secretKey:            []byte(secretKey),
		accessTokenDuration:  accessTokenDuration,
		refreshTokenDuration: refreshTokenDuration,
		issuer:               "gin-clean-starter",
		audience:             "gin-clean-starter-api",
		keyID:                "default",
	}
}

func (s *Service) GenerateAccessToken(userID uuid.UUID, role user.Role) (string, error) {
	return s.generateToken(userID, role, TokenTypeAccess, s.accessTokenDuration)
}

func (s *Service) GenerateRefreshToken(userID uuid.UUID, role user.Role) (string, error) {
	return s.generateToken(userID, role, TokenTypeRefresh, s.refreshTokenDuration)
}

func (s *Service) GetAccessTokenDuration() time.Duration {
	return s.accessTokenDuration
}

func (s *Service) GetRefreshTokenDuration() time.Duration {
	return s.refreshTokenDuration
}

func (s *Service) generateToken(userID uuid.UUID, role user.Role, tokenType TokenType, duration time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:    userID,
		Role:      role.String(),
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    s.issuer,
			Audience:  []string{s.audience},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
			ID:        uuid.NewString(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token.Header["kid"] = s.keyID
	return token.SignedString(s.secretKey)
}

func (s *Service) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, ErrInvalidToken
	}

	if claims.Issuer != s.issuer {
		return nil, ErrInvalidToken
	}

	validAudience := false
	for _, aud := range claims.Audience {
		if aud == s.audience {
			validAudience = true
			break
		}
	}
	if !validAudience {
		return nil, ErrInvalidToken
	}

	return claims, nil
}
