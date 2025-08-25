//go:build unit || e2e

package authtest

import (
	"testing"
	"time"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type JWTHelper struct {
	cfg config.JWTConfig
}

func NewJWTHelper(cfg config.JWTConfig) *JWTHelper {
	return &JWTHelper{cfg: cfg}
}

func (h *JWTHelper) GenerateToken(t *testing.T, userID uuid.UUID, role user.Role) string {
	t.Helper()
	duration, err := time.ParseDuration(h.cfg.AccessTokenDuration)
	require.NoError(t, err)
	refreshDuration, err := time.ParseDuration(h.cfg.RefreshTokenDuration)
	require.NoError(t, err)
	service := jwt.NewService(h.cfg.Secret, duration, refreshDuration)
	token, err := service.GenerateAccessToken(userID, role)
	require.NoError(t, err)
	return token
}

func (h *JWTHelper) CreateExpiredToken(t *testing.T, userID uuid.UUID, role user.Role) string {
	t.Helper()
	refreshDuration, err := time.ParseDuration(h.cfg.RefreshTokenDuration)
	require.NoError(t, err)
	service := jwt.NewService(h.cfg.Secret, 1*time.Millisecond, refreshDuration)
	token, err := service.GenerateAccessToken(userID, role)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	return token
}
