//go:build unit || e2e

package authtest

import (
	"net/http"
	"testing"

	"gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/tests/common/dbtest"
	"gin-clean-starter/tests/common/httptest"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func LoginUser(t *testing.T, router *gin.Engine, email, password string) string {
	t.Helper()

	w := httptest.PerformRequest(t, router, http.MethodPost, "/api/auth/login",
		request.LoginRequest{Email: email, Password: password}, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Extract access token from cookie
	accessCookie := httptest.ExtractCookie(w, "access_token")
	require.NotNil(t, accessCookie, "Access token not found in cookies")
	require.NotEmpty(t, accessCookie.Value, "Access token cookie is empty")

	return accessCookie.Value
}

func CreateAndLogin(t *testing.T, db dbtest.DBLike, router *gin.Engine, email, role string) string {
	t.Helper()
	dbtest.CreateTestUser(t, db, email, role)
	return LoginUser(t, router, email, "password123")
}

func LogoutUser(t *testing.T, router *gin.Engine, cookies []*http.Cookie) {
	t.Helper()

	w := httptest.PerformRequestWithCookies(t, router, http.MethodPost, "/api/auth/logout", nil, cookies, "")
	require.Equal(t, http.StatusNoContent, w.Code, w.Body.String())
}
