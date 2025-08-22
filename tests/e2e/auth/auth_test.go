//go:build e2e

package auth_test

import (
	"net/http"
	"testing"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/tests/common/helper"
	"gin-clean-starter/tests/e2e"
	jwtHelper "gin-clean-starter/tests/e2e/common/helper"

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
	jwtHelper *jwtHelper.JWTTestHelper
}

func TestAuthSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(authSuite))
}

func (s *authSuite) SetupSuite() {
	s.SharedSuite.SetupSuite()
	s.jwtHelper = jwtHelper.NewJWTTestHelper(s.GetBaseDB(), s.Config.JWT)
}

func (s *authSuite) SetupSubTest() {
	s.SharedSuite.SetupSubTest()

	// テスト用ユーザーを作成
	s.jwtHelper.CreateTestUserWithDB(s.T(), s.DB, "test@example.com", string(user.RoleAdmin))
	s.jwtHelper.CreateTestUserWithDB(s.T(), s.DB, "viewer@example.com", string(user.RoleViewer))
	s.jwtHelper.CreateTestUserWithDB(s.T(), s.DB, "operator@example.com", string(user.RoleOperator))
	s.jwtHelper.CreateTestUserWithDB(s.T(), s.DB, "inactive@example.com", string(user.RoleAdmin))

	// 非アクティブユーザーを作成
	ctx := s.T().Context()
	_, err := s.DB.Exec(ctx, "UPDATE users SET is_active = false WHERE email = 'inactive@example.com'")
	require.NoError(s.T(), err)
}

func (s *authSuite) TestLogin() {
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
			expectedStatus: http.StatusUnauthorized,
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

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			reqBody := request.LoginRequest{
				Email:    tt.email,
				Password: tt.password,
			}

			w := helper.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				// 成功時のレスポンス形式チェック
				var loginRes jwtHelper.LoginResponse
				err := helper.DecodeResponseBody(t, w.Body, &loginRes)
				require.NoError(t, err)
				require.NotEmpty(t, loginRes.AccessToken, "アクセストークンが空")
				require.NotEmpty(t, loginRes.RefreshToken, "リフレッシュトークンが空")
				require.Greater(t, loginRes.ExpiresIn, int64(0), "有効期限が無効")

				// last_loginが更新されることを確認
				var lastLogin interface{}
				err = s.DB.QueryRow(s.T().Context(), "SELECT last_login FROM users WHERE email = $1", tt.email).Scan(&lastLogin)
				require.NoError(t, err)
				require.NotNil(t, lastLogin, "last_loginが更新されていない")
			}
		})
	}
}

func (s *authSuite) TestRefresh() {
	tests := []struct {
		name              string
		setupRefreshToken func() string
		expectedStatus    int
		description       string
	}{
		{
			name: "正常なリフレッシュ",
			setupRefreshToken: func() string {
				// リフレッシュトークンを取得するため再度ログイン
				reqBody := request.LoginRequest{
					Email:    "test@example.com",
					Password: "password123",
				}
				w := helper.PerformRequest(s.T(), s.Router, http.MethodPost, loginURL, reqBody, "")
				var loginRes jwtHelper.LoginResponse
				helper.DecodeResponseBody(s.T(), w.Body, &loginRes)
				return loginRes.RefreshToken
			},
			expectedStatus: http.StatusOK,
			description:    "有効なリフレッシュトークンでトークンが更新されること",
		},
		{
			name: "無効なリフレッシュトークン",
			setupRefreshToken: func() string {
				return "invalid-refresh-token"
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "無効なリフレッシュトークンは拒否されること",
		},
		{
			name: "空のリフレッシュトークン",
			setupRefreshToken: func() string {
				return ""
			},
			expectedStatus: http.StatusBadRequest,
			description:    "空のリフレッシュトークンは拒否されること",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			refreshToken := tt.setupRefreshToken()
			reqBody := request.RefreshRequest{
				RefreshToken: refreshToken,
			}

			w := helper.PerformRequest(t, s.Router, http.MethodPost, refreshURL, reqBody, "")
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				var refreshRes jwtHelper.LoginResponse
				err := helper.DecodeResponseBody(t, w.Body, &refreshRes)
				require.NoError(t, err)
				require.NotEmpty(t, refreshRes.AccessToken, "新しいアクセストークンが空")
				require.NotEmpty(t, refreshRes.RefreshToken, "新しいリフレッシュトークンが空")
			}
		})
	}
}

func (s *authSuite) TestLogout() {
	tests := []struct {
		name           string
		setupToken     func() string
		expectedStatus int
		description    string
	}{
		{
			name: "正常なログアウト",
			setupToken: func() string {
				return s.jwtHelper.LoginUser(s.T(), s.Router, "test@example.com", "password123")
			},
			expectedStatus: http.StatusOK,
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
			w := helper.PerformRequest(t, s.Router, http.MethodPost, logoutURL, nil, token)
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)
		})
	}
}

func (s *authSuite) TestMe() {
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
				token := s.jwtHelper.CreateAndLoginWithDB(s.T(), s.DB, s.Router, email, role)
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
				token := s.jwtHelper.CreateAndLoginWithDB(s.T(), s.DB, s.Router, email, role)
				return email, role, token
			},
			expectedStatus: http.StatusOK,
			description:    "Viewerユーザーの情報が取得できること",
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
			w := helper.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token)
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

		// テスト用のユーザーIDを取得
		userID := s.jwtHelper.CreateTestUser(t, "expiry@example.com", string(user.RoleAdmin))

		// 期限切れトークンを作成
		expiredToken := s.jwtHelper.CreateExpiredToken(t, userID, user.RoleAdmin)

		// 期限切れトークンでアクセスを試行
		w := helper.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, expiredToken)
		require.Equal(t, http.StatusUnauthorized, w.Code, "期限切れトークンは拒否されるべき")
	})
}

func (s *authSuite) TestAuthenticationRequired() {
	s.Run("認証が必要なエンドポイント", func() {
		t := s.T()

		endpoints := []struct {
			method string
			path   string
		}{
			{http.MethodPost, logoutURL},
			{http.MethodGet, meURL},
		}

		for _, endpoint := range endpoints {
			w := helper.PerformRequest(t, s.Router, endpoint.method, endpoint.path, nil, "")
			require.Equal(t, http.StatusUnauthorized, w.Code, "認証なしでは拒否されるべき")
		}
	})
}

func (s *authSuite) TestConcurrentLogin() {
	s.Run("同時ログイン", func() {
		t := s.T()

		email := "concurrent@example.com"
		s.jwtHelper.CreateTestUser(t, email, string(user.RoleAdmin))

		// 複数回ログイン
		token1 := s.jwtHelper.LoginUser(t, s.Router, email, "password123")
		token2 := s.jwtHelper.LoginUser(t, s.Router, email, "password123")

		require.NotEqual(t, token1, token2, "同時ログインで同じトークンが返された")

		// 両方のトークンが有効であることを確認
		w1 := helper.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token1)
		w2 := helper.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token2)

		require.Equal(t, http.StatusOK, w1.Code, "最初のトークンが無効")
		require.Equal(t, http.StatusOK, w2.Code, "二番目のトークンが無効")
	})
}
