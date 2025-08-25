//go:build e2e

package auth_test

import (
	"context"
	"net/http"
	"testing"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/tests/common/authtest"
	"gin-clean-starter/tests/common/dbtest"
	"gin-clean-starter/tests/common/httptest"
	"gin-clean-starter/tests/common/testutil"
	"gin-clean-starter/tests/e2e"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	loginURL   = "/api/auth/login"
	logoutURL  = "/api/auth/logout"
	refreshURL = "/api/auth/refresh"
	meURL      = "/api/auth/me"
)

type authSuite struct {
	e2e.SharedSuite
	jwtHelper *authtest.JWTHelper
}

func TestAuthSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(authSuite))
}

func (s *authSuite) SetupSuite() {
	s.SharedSuite.SetupSuite()
	s.jwtHelper = authtest.NewJWTHelper(s.Config.JWT)
}

func (s *authSuite) SetupSubTest() {
	s.SharedSuite.SetupSubTest()

	dbtest.CreateTestUser(s.T(), s.DB, "test@example.com", string(user.RoleAdmin))
	dbtest.CreateTestUser(s.T(), s.DB, "viewer@example.com", string(user.RoleViewer))
	dbtest.CreateTestUser(s.T(), s.DB, "operator@example.com", string(user.RoleOperator))
	dbtest.CreateTestUser(s.T(), s.DB, "inactive@example.com", string(user.RoleAdmin))
	ctx := s.T().Context()
	_, err := s.DB.Exec(ctx, "UPDATE users SET is_active = false WHERE email = 'inactive@example.com'")
	require.NoError(s.T(), err)
}

func (s *authSuite) TestLogin() {
	s.SetupSubTest() // Ensure clean state for table-driven tests

	tests := []struct {
		name           string
		email          string
		password       string
		expectedStatus int
		description    string
	}{
		{
			name:           "正常なログイン",
			email:          "test@example.com",
			password:       "password123",
			expectedStatus: http.StatusOK,
			description:    "有効な認証情報でログインできること",
		},
		{
			name:           "存在しないユーザー",
			email:          "nonexistent@example.com",
			password:       "password123",
			expectedStatus: http.StatusUnauthorized,
			description:    "存在しないユーザーでログインできないこと",
		},
		{
			name:           "間違ったパスワード",
			email:          "test@example.com",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			description:    "間違ったパスワードでログインできないこと",
		},
		{
			name:           "非アクティブユーザー",
			email:          "inactive@example.com",
			password:       "password123",
			expectedStatus: http.StatusForbidden,
			description:    "非アクティブユーザーはログインできないこと",
		},
		{
			name:           "空のメールアドレス",
			email:          "",
			password:       "password123",
			expectedStatus: http.StatusBadRequest,
			description:    "空のメールアドレスは拒否されること",
		},
		{
			name:           "空のパスワード",
			email:          "test@example.com",
			password:       "",
			expectedStatus: http.StatusBadRequest,
			description:    "空のパスワードは拒否されること",
		},
	}

	s.Run("動的データ構築テスト", func() {
		t := s.T()

		baseData := map[string]any{
			"email":    "",
			"password": "",
		}

		testutil.Field("email", "test@example.com")(baseData)
		testutil.Field("password", "password123")(baseData)

		reqBody := request.LoginRequest{
			Email:    baseData["email"].(string),
			Password: baseData["password"].(string),
		}

		w := httptest.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
		require.Equal(t, http.StatusOK, w.Code)
	})

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			reqBody := request.LoginRequest{
				Email:    tt.email,
				Password: tt.password,
			}

			w := httptest.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				var loginRes struct {
					User struct {
						Email string `json:"email"`
					} `json:"user"`
				}
				err := httptest.DecodeResponseBody(t, w.Body, &loginRes)
				require.NoError(t, err)
				require.Equal(t, tt.email, loginRes.User.Email, "ユーザー情報が正しくない")

				accessCookie := httptest.ExtractCookie(w, "access_token")
				refreshCookie := httptest.ExtractCookie(w, "refresh_token")
				require.NotNil(t, accessCookie, "アクセストークンがクッキーに設定されていない")
				require.NotNil(t, refreshCookie, "リフレッシュトークンがクッキーに設定されていない")
				require.NotEmpty(t, accessCookie.Value, "アクセストークンが空です")
				require.NotEmpty(t, refreshCookie.Value, "リフレッシュトークンが空です")

				var lastLogin any
				err = s.DB.QueryRow(s.T().Context(), "SELECT last_login FROM users WHERE email = $1", tt.email).Scan(&lastLogin)
				require.NoError(t, err)
				require.NotNil(t, lastLogin, "last_loginが更新されていない")
			}
		})
	}
}

func (s *authSuite) TestRefresh() {
	s.SetupSubTest()

	tests := []struct {
		name           string
		setupCookies   func(t *testing.T) []*http.Cookie
		expectedStatus int
		description    string
	}{
		{
			name: "正常なリフレッシュ",
			setupCookies: func(t *testing.T) []*http.Cookie {
				// ログインしてリフレッシュトークンを含むクッキーを取得
				reqBody := request.LoginRequest{
					Email:    "test@example.com",
					Password: "password123",
				}
				w := httptest.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
				require.Equal(t, http.StatusOK, w.Code)
				return httptest.ExtractCookies(w)
			},
			expectedStatus: http.StatusOK,
			description:    "有効なリフレッシュトークンでトークンが更新されること",
		},
		{
			name: "無効なリフレッシュトークン",
			setupCookies: func(t *testing.T) []*http.Cookie {
				return []*http.Cookie{
					{Name: "refresh_token", Value: "invalid-refresh-token"},
				}
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "無効なリフレッシュトークンは拒否されること",
		},
		{
			name: "空のリフレッシュトークン",
			setupCookies: func(t *testing.T) []*http.Cookie {
				return []*http.Cookie{} // No cookies
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "リフレッシュトークンクッキーがない場合は拒否されること",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			cookies := tt.setupCookies(t)

			// Create request with cookies using new helper function
			w := httptest.PerformRequestWithCookies(t, s.Router, http.MethodPost, refreshURL, nil, cookies, "")
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				// Check that response contains success message
				var refreshRes struct {
					Message string `json:"message"`
				}
				err := httptest.DecodeResponseBody(t, w.Body, &refreshRes)
				require.NoError(t, err)
				require.NotEmpty(t, refreshRes.Message)

				// Check that new tokens are set in cookies
				cookies := w.Result().Cookies()
				var newAccessToken, newRefreshToken string
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						newAccessToken = cookie.Value
					} else if cookie.Name == "refresh_token" {
						newRefreshToken = cookie.Value
					}
				}
				require.NotEmpty(t, newAccessToken, "新しいアクセストークンがクッキーに設定されていない")
				require.NotEmpty(t, newRefreshToken, "新しいリフレッシュトークンがクッキーに設定されていない")
			}
		})
	}
}

func (s *authSuite) TestLogout() {
	s.Run("クッキーベースのログアウト", func() {
		t := s.T()

		reqBody := request.LoginRequest{
			Email:    "test@example.com",
			Password: "password123",
		}
		w := httptest.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
		require.Equal(t, http.StatusOK, w.Code)

		cookies := httptest.ExtractCookies(w)
		authtest.LogoutUser(t, s.Router, cookies)
	})

	s.SetupSubTest()
	tests := []struct {
		name           string
		setupToken     func() string
		expectedStatus int
		description    string
	}{
		{
			name: "正常なログアウト",
			setupToken: func() string {
				return authtest.LoginUser(s.T(), s.Router, "test@example.com", "password123")
			},
			expectedStatus: http.StatusNoContent,
			description:    "有効なトークンでログアウトできること",
		},
		{
			name: "無効なトークン",
			setupToken: func() string {
				return "invalid-token"
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "無効なトークンでログアウトできないこと",
		},
		{
			name: "トークンなし",
			setupToken: func() string {
				return ""
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "トークンなしでログアウトできないこと",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			token := tt.setupToken()
			w := httptest.PerformRequest(t, s.Router, http.MethodPost, logoutURL, nil, token)
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

func (s *authSuite) TestMe() {
	s.SetupSubTest()

	tests := []struct {
		name           string
		setupUser      func() (string, string, string) // email, role, token
		expectedStatus int
		description    string
	}{
		{
			name: "管理者ユーザーの情報取得",
			setupUser: func() (string, string, string) {
				email := "admin@example.com"
				role := string(user.RoleAdmin)
				token := authtest.CreateAndLogin(s.T(), s.DB, s.Router, email, role)
				return email, role, token
			},
			expectedStatus: http.StatusOK,
			description:    "管理者ユーザーの情報が取得できること",
		},
		{
			name: "Viewerユーザーの情報取得",
			setupUser: func() (string, string, string) {
				email := "viewer2@example.com"
				role := string(user.RoleViewer)
				token := authtest.CreateAndLogin(s.T(), s.DB, s.Router, email, role)
				return email, role, token
			},
			expectedStatus: http.StatusOK,
			description:    "Viewerユーザーの情報が取得できること",
		},
		{
			name: "カスタム会社のユーザー情報取得",
			setupUser: func() (string, string, string) {
				companyID := dbtest.CreateTestCompany(s.T(), s.DB, "Custom Test Corp")
				email := "custom@testcorp.com"
				userID := uuid.New()
				passwordHash := "$2a$12$uhAjVE9f92IGYv3E25pJNetg.27lVt0p7jmLWjqjmhOg92ldPS0A."
				ctx := context.Background()
				_, err := s.DB.Exec(ctx, "INSERT INTO users (id, email, password_hash, role, company_id, is_active) VALUES ($1, $2, $3, $4, $5, true)",
					userID, email, passwordHash, string(user.RoleAdmin), companyID)
				require.NoError(s.T(), err)
				token := authtest.LoginUser(s.T(), s.Router, email, "password123")
				return email, string(user.RoleAdmin), token
			},
			expectedStatus: http.StatusOK,
			description:    "カスタム会社に所属するユーザーの情報が取得できること",
		},
		{
			name: "無効なトークン",
			setupUser: func() (string, string, string) {
				return "", "", "invalid-token"
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "無効なトークンでは情報取得できないこと",
		},
		{
			name: "トークンなし",
			setupUser: func() (string, string, string) {
				return "", "", ""
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "トークンなしでは情報取得できないこと",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			email, role, token := tt.setupUser()
			w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token)
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				// レスポンス内容をチェック
				responseBody := w.Body.String()
				require.Contains(t, responseBody, email, "レスポンスにメールアドレスが含まれていない")
				require.Contains(t, responseBody, role, "レスポンスにロールが含まれていない")
				require.NotContains(t, responseBody, "password", "レスポンスにパスワード情報が含まれている")
			}
		})
	}
}

func (s *authSuite) TestTokenExpiry() {
	s.Run("期限切れトークンの拒否", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "expiry@example.com", string(user.RoleAdmin))
		expiredToken := s.jwtHelper.CreateExpiredToken(t, userID, user.RoleAdmin)

		w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, expiredToken)
		require.Equal(t, http.StatusUnauthorized, w.Code, "期限切れトークンは拒否されるべき")
	})

	s.Run("有効なトークンの受け入れ", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "valid@example.com", string(user.RoleAdmin))
		validToken := s.jwtHelper.GenerateToken(t, userID, user.RoleAdmin)

		w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, validToken)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

func (s *authSuite) TestAuthenticationRequired() {
	s.Run("認証が必要なエンドポイント", func() {
		s.SetupSubTest()
		t := s.T()

		endpoints := []struct {
			method string
			path   string
		}{
			{http.MethodPost, logoutURL},
			{http.MethodGet, meURL},
		}

		for _, endpoint := range endpoints {
			w := httptest.PerformRequest(t, s.Router, endpoint.method, endpoint.path, nil, "")
			require.Equal(t, http.StatusUnauthorized, w.Code, "認証なしでは拒否されるべき")
		}
	})
}

func (s *authSuite) TestConcurrentLogin() {
	s.Run("同時ログイン", func() {
		t := s.T()

		email := "concurrent@example.com"
		dbtest.CreateTestUser(t, s.DB, email, string(user.RoleAdmin))

		// 複数回ログイン
		token1 := authtest.LoginUser(t, s.Router, email, "password123")
		token2 := authtest.LoginUser(t, s.Router, email, "password123")

		require.NotEqual(t, token1, token2, "同時ログインで同じトークンが返された")

		// 両方のトークンが有効であることを確認
		w1 := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token1)
		w2 := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token2)

		require.Equal(t, http.StatusOK, w1.Code, "最初のトークンが無効")
		require.Equal(t, http.StatusOK, w2.Code, "二番目のトークンが無効")
	})
}
