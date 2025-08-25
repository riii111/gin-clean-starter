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
			name:           "Successful login",
			email:          "test@example.com",
			password:       "password123",
			expectedStatus: http.StatusOK,
			description:    "Should login with valid credentials",
		},
		{
			name:           "Nonexistent user",
			email:          "nonexistent@example.com",
			password:       "password123",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject nonexistent user",
		},
		{
			name:           "Wrong password",
			email:          "test@example.com",
			password:       "wrongpassword",
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject wrong password",
		},
		{
			name:           "Inactive user",
			email:          "inactive@example.com",
			password:       "password123",
			expectedStatus: http.StatusForbidden,
			description:    "Should reject inactive user",
		},
		{
			name:           "Empty email",
			email:          "",
			password:       "password123",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty email",
		},
		{
			name:           "Empty password",
			email:          "test@example.com",
			password:       "",
			expectedStatus: http.StatusBadRequest,
			description:    "Should reject empty password",
		},
	}

	s.Run("Dynamic data construction test", func() {
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
				require.Equal(t, tt.email, loginRes.User.Email, "User info is incorrect")

				accessCookie := httptest.ExtractCookie(w, "access_token")
				refreshCookie := httptest.ExtractCookie(w, "refresh_token")
				require.NotNil(t, accessCookie, "Access token not set in cookie")
				require.NotNil(t, refreshCookie, "Refresh token not set in cookie")
				require.NotEmpty(t, accessCookie.Value, "Access token is empty")
				require.NotEmpty(t, refreshCookie.Value, "Refresh token is empty")

				var lastLogin any
				err = s.DB.QueryRow(s.T().Context(), "SELECT last_login FROM users WHERE email = $1", tt.email).Scan(&lastLogin)
				require.NoError(t, err)
				require.NotNil(t, lastLogin, "last_login not updated")
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
			name: "Valid refresh",
			setupCookies: func(t *testing.T) []*http.Cookie {
				// Login and get cookies with refresh token
				reqBody := request.LoginRequest{
					Email:    "test@example.com",
					Password: "password123",
				}
				w := httptest.PerformRequest(t, s.Router, http.MethodPost, loginURL, reqBody, "")
				require.Equal(t, http.StatusOK, w.Code)
				return httptest.ExtractCookies(w)
			},
			expectedStatus: http.StatusOK,
			description:    "Should refresh tokens with valid refresh token",
		},
		{
			name: "Invalid refresh token",
			setupCookies: func(t *testing.T) []*http.Cookie {
				return []*http.Cookie{
					{Name: "refresh_token", Value: "invalid-refresh-token"},
				}
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject invalid refresh token",
		},
		{
			name: "Empty refresh token",
			setupCookies: func(t *testing.T) []*http.Cookie {
				return []*http.Cookie{} // No cookies
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject when refresh token cookie is missing",
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
				// Check response contains success message
				var refreshRes struct {
					Message string `json:"message"`
				}
				err := httptest.DecodeResponseBody(t, w.Body, &refreshRes)
				require.NoError(t, err)
				require.NotEmpty(t, refreshRes.Message)

				// Check new tokens are set in cookies
				cookies := w.Result().Cookies()
				var newAccessToken, newRefreshToken string
				for _, cookie := range cookies {
					if cookie.Name == "access_token" {
						newAccessToken = cookie.Value
					} else if cookie.Name == "refresh_token" {
						newRefreshToken = cookie.Value
					}
				}
				require.NotEmpty(t, newAccessToken, "New access token not set in cookie")
				require.NotEmpty(t, newRefreshToken, "New refresh token not set in cookie")
			}
		})
	}
}

func (s *authSuite) TestLogout() {
	s.Run("Cookie-based logout", func() {
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
			name: "Valid logout",
			setupToken: func() string {
				return authtest.LoginUser(s.T(), s.Router, "test@example.com", "password123")
			},
			expectedStatus: http.StatusNoContent,
			description:    "Should logout with valid token",
		},
		{
			name: "Invalid token",
			setupToken: func() string {
				return "invalid-token"
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject invalid token",
		},
		{
			name: "No token",
			setupToken: func() string {
				return ""
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject when no token provided",
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
			name: "Admin user info",
			setupUser: func() (string, string, string) {
				email := "admin@example.com"
				role := string(user.RoleAdmin)
				token := authtest.CreateAndLogin(s.T(), s.DB, s.Router, email, role)
				return email, role, token
			},
			expectedStatus: http.StatusOK,
			description:    "Should get admin user info",
		},
		{
			name: "Viewer user info",
			setupUser: func() (string, string, string) {
				email := "viewer2@example.com"
				role := string(user.RoleViewer)
				token := authtest.CreateAndLogin(s.T(), s.DB, s.Router, email, role)
				return email, role, token
			},
			expectedStatus: http.StatusOK,
			description:    "Should get viewer user info",
		},
		{
			name: "Custom company user info",
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
			description:    "Should get user info for custom company",
		},
		{
			name: "Invalid token",
			setupUser: func() (string, string, string) {
				return "", "", "invalid-token"
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject invalid token",
		},
		{
			name: "No token",
			setupUser: func() (string, string, string) {
				return "", "", ""
			},
			expectedStatus: http.StatusUnauthorized,
			description:    "Should reject when no token provided",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			t := s.T()

			email, role, token := tt.setupUser()
			w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token)
			require.Equal(t, tt.expectedStatus, w.Code, tt.description)

			if tt.expectedStatus == http.StatusOK {
				// Check response content
				responseBody := w.Body.String()
				require.Contains(t, responseBody, email, "Response should contain email")
				require.Contains(t, responseBody, role, "Response should contain role")
				require.NotContains(t, responseBody, "password", "Response should not contain password")
			}
		})
	}
}

func (s *authSuite) TestTokenExpiry() {
	s.Run("Expired token rejection", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "expiry@example.com", string(user.RoleAdmin))
		expiredToken := s.jwtHelper.CreateExpiredToken(t, userID, user.RoleAdmin)

		w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, expiredToken)
		require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject expired token")
	})

	s.Run("Valid token acceptance", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "valid@example.com", string(user.RoleAdmin))
		validToken := s.jwtHelper.GenerateToken(t, userID, user.RoleAdmin)

		w := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, validToken)
		require.Equal(t, http.StatusOK, w.Code)
	})
}

func (s *authSuite) TestAuthenticationRequired() {
	s.Run("Authentication required endpoints", func() {
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
			require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject without authentication")
		}
	})
}

func (s *authSuite) TestConcurrentLogin() {
	s.Run("Concurrent login", func() {
		t := s.T()

		email := "concurrent@example.com"
		dbtest.CreateTestUser(t, s.DB, email, string(user.RoleAdmin))

		// Multiple logins
		token1 := authtest.LoginUser(t, s.Router, email, "password123")
		token2 := authtest.LoginUser(t, s.Router, email, "password123")

		require.NotEqual(t, token1, token2, "Same token returned for concurrent login")

		// Verify both tokens are valid
		w1 := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token1)
		w2 := httptest.PerformRequest(t, s.Router, http.MethodGet, meURL, nil, token2)

		require.Equal(t, http.StatusOK, w1.Code, "First token is invalid")
		require.Equal(t, http.StatusOK, w2.Code, "Second token is invalid")
	})
}
