//go:build e2e

package helper

import (
	"context"
	"net/http"
	"testing"
	"time"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/tests/common/helper"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type JWTTestHelper struct {
	pool *pgxpool.Pool
	cfg  config.JWTConfig
}

func NewJWTTestHelper(pool *pgxpool.Pool, cfg config.JWTConfig) *JWTTestHelper {
	return &JWTTestHelper{pool: pool, cfg: cfg}
}

// DBLike is the minimal interface required for test DB operations.
type DBLike interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (h *JWTTestHelper) CreateTestUser(t *testing.T, email, role string) uuid.UUID {
	// Backward-compatible: uses base pool (committed). Prefer CreateTestUserWithDB in tests.
	return h.CreateTestUserWithDB(t, h.pool, email, role)
}

func (h *JWTTestHelper) CreateTestUserWithDB(t *testing.T, db DBLike, email, role string) uuid.UUID {
	t.Helper()

	userID := uuid.New()
	var companyID uuid.UUID

	ctx := context.Background()
	err := db.QueryRow(ctx, "SELECT id FROM companies WHERE name = 'Default Company' LIMIT 1").Scan(&companyID)
	require.NoError(t, err)

	passwordHash := "$2a$12$uhAjVE9f92IGYv3E25pJNetg.27lVt0p7jmLWjqjmhOg92ldPS0A."
	tag, err := db.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, company_id, is_active) VALUES ($1, $2, $3, $4, $5, true) ON CONFLICT (email) WHERE is_active = true DO NOTHING",
		userID, email, passwordHash, role, companyID)
	require.NoError(t, err)

	if tag.RowsAffected() == 0 {
		// Already exists and active; fetch existing id to keep deterministic behavior
		_ = db.QueryRow(ctx, "SELECT id FROM users WHERE email = $1 AND is_active = true", email).Scan(&userID)
	}

	return userID
}

func (h *JWTTestHelper) LoginUser(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()

	w := helper.PerformRequest(t, router, http.MethodPost, "/api/auth/login",
		request.LoginRequest{Email: email, Password: password}, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Extract access token from cookie instead of JSON response
	cookies := w.Result().Cookies()
	var accessToken string
	for _, cookie := range cookies {
		if cookie.Name == "access_token" {
			accessToken = cookie.Value
			break
		}
	}
	require.NotEmpty(t, accessToken, "Access token not found in cookies")

	return accessToken
}

func (h *JWTTestHelper) CreateAndLogin(t *testing.T, router *gin.Engine, email, role string) string {
	t.Helper()
	h.CreateTestUserWithDB(t, h.pool, email, role)
	return h.LoginUser(t, router, email, "password123")
}

func (h *JWTTestHelper) CreateAndLoginWithDB(t *testing.T, db DBLike, router *gin.Engine, email, role string) string {
	t.Helper()
	h.CreateTestUserWithDB(t, db, email, role)
	return h.LoginUser(t, router, email, "password123")
}

func (h *JWTTestHelper) GenerateToken(t *testing.T, userID uuid.UUID, role user.Role) string {
	t.Helper()
	duration, _ := time.ParseDuration(h.cfg.AccessTokenDuration)
	refreshDuration, _ := time.ParseDuration(h.cfg.RefreshTokenDuration)
	service := jwt.NewService(h.cfg.Secret, duration, refreshDuration)
	token, err := service.GenerateAccessToken(userID, role)
	require.NoError(t, err)
	return token
}

func (h *JWTTestHelper) CreateExpiredToken(t *testing.T, userID uuid.UUID, role user.Role) string {
	t.Helper()
	refreshDuration, _ := time.ParseDuration(h.cfg.RefreshTokenDuration)
	service := jwt.NewService(h.cfg.Secret, 1*time.Millisecond, refreshDuration)
	token, err := service.GenerateAccessToken(userID, role)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	return token
}
