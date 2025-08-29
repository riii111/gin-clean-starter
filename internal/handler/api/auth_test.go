//go:build unit

package api_test

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"gin-clean-starter/internal/handler/api"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/usecase/commands"
	"gin-clean-starter/internal/usecase/queries"
	"gin-clean-starter/tests/common/builder"
	"gin-clean-starter/tests/common/httptest"
	"gin-clean-starter/tests/common/testutil"
	commandsmock "gin-clean-starter/tests/mock/commands"
	queriesmock "gin-clean-starter/tests/mock/queries"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type AuthHandlerTestSuite struct {
	suite.Suite
	router       *gin.Engine
	mockCtrl     *gomock.Controller
	mockCommands *commandsmock.MockAuthCommands
	mockQueries  *queriesmock.MockUserQueries
	handler      *api.AuthHandler
}

func (s *AuthHandlerTestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	s.mockCtrl = gomock.NewController(s.T())
	s.mockCommands = commandsmock.NewMockAuthCommands(s.mockCtrl)
	s.mockQueries = queriesmock.NewMockUserQueries(s.mockCtrl)
	mockJWTService := &jwt.Service{} // Mock JWT service for testing
	s.handler = api.NewAuthHandler(s.mockCommands, s.mockQueries, mockJWTService, config.NewTestConfig())

	s.router.POST("/auth/login", s.handler.Login)
	s.router.POST("/auth/logout", s.handler.Logout)
	s.router.GET("/auth/me", func(c *gin.Context) {
		// Mock middleware behavior for /auth/me
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			c.Set("user_id", uuid.New())
		}
		s.handler.Me(c)
	})
}

func (s *AuthHandlerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func TestAuthHandlerSuite(t *testing.T) {
	suite.Run(t, new(AuthHandlerTestSuite))
}

type testCaseAuth struct {
	name         string
	mutate       func(m map[string]any)
	expectCode   int
	expectInBody string
}

func (s *AuthHandlerTestSuite) TestLogin() {
	url := "/auth/login"

	reqBody := builder.NewAuthBuilder().BuildDTO()
	returnUser := builder.NewUserBuilder().BuildReadModel()
	expectedToken := "test-jwt-token"
	expectedRefresh := "test-refresh-token"

	s.Run("success: returns 200 OK for valid credentials", func() {
		s.mockCommands.EXPECT().Login(gomock.Any(), reqBody).
			Return(&commands.LoginResult{
				UserID:     returnUser.ID,
				TokenPair:  &commands.TokenPair{AccessToken: expectedToken, RefreshToken: expectedRefresh},
				IsReplayed: false,
			}, nil).Times(1)
		s.mockQueries.EXPECT().GetCurrentUser(gomock.Any(), returnUser.ID).
			Return(returnUser, nil).Times(1)
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")

		var response resdto.LoginResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(returnUser.Email, response.User.Email)
	})

	s.Run("error: 400 Bad Request on validation errors", func() {
		bound := []testCaseAuth{
			{name: "email boundary OK (valid email)", mutate: testutil.Field("email", "valid@example.com"), expectCode: http.StatusOK},
			{name: "email boundary invalid (invalid email)", mutate: testutil.Field("email", "invalid-email"), expectCode: http.StatusBadRequest},
			{name: "password boundary OK (8 chars)", mutate: testutil.Field("password", "password"), expectCode: http.StatusOK},
			{name: "password boundary invalid (7 chars)", mutate: testutil.Field("password", strings.Repeat("a", 7)), expectCode: http.StatusBadRequest},
		}

		missing := []testCaseAuth{
			{name: "missing field: email (required)", mutate: testutil.Field("email", nil), expectCode: http.StatusBadRequest},
			{name: "missing field: password (required)", mutate: testutil.Field("password", nil), expectCode: http.StatusBadRequest},
		}

		empty := []testCaseAuth{
			{name: "empty email", mutate: testutil.Field("email", ""), expectCode: http.StatusBadRequest},
			{name: "empty password", mutate: testutil.Field("password", ""), expectCode: http.StatusBadRequest},
		}

		allValidationTestCases := [][]testCaseAuth{bound, missing, empty}

		for _, testCaseGroup := range allValidationTestCases {
			for _, tc := range testCaseGroup {
				s.Run(tc.name, func() {
					requestMap := testutil.DtoMap(s.T(), reqBody, tc.mutate)

					if tc.expectCode == http.StatusOK {
						email, _ := requestMap["email"].(string)
						password, _ := requestMap["password"].(string)
						expectedReq := (&builder.AuthBuilder{Email: email, Password: password}).BuildDTO()
						s.mockCommands.EXPECT().Login(gomock.Any(), expectedReq).
							Return(&commands.LoginResult{
								UserID:     returnUser.ID,
								TokenPair:  &commands.TokenPair{AccessToken: expectedToken, RefreshToken: expectedRefresh},
								IsReplayed: false,
							}, nil)
						s.mockQueries.EXPECT().GetCurrentUser(gomock.Any(), returnUser.ID).
							Return(returnUser, nil)
					}
					rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, requestMap, "")
					if tc.expectCode == http.StatusOK {
						httptest.AssertSuccessResponse(s.T(), rec, tc.expectCode, nil)
					} else {
						httptest.AssertErrorResponse(s.T(), rec, tc.expectCode, "")
					}
				})
			}
		}
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "invalid credentials",
				commandsError:  commands.ErrInvalidCredentials,
				expectedStatus: http.StatusUnauthorized,
				expectedMsg:    "Invalid email or password",
			},
			{
				name:           "user not found",
				commandsError:  commands.ErrUserNotFound,
				expectedStatus: http.StatusUnauthorized,
				expectedMsg:    "Invalid email or password",
			},
			{
				name:           "user inactive",
				commandsError:  commands.ErrUserInactive,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Account is inactive",
			},
			{
				name:           "internal server error",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal server error",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockCommands.EXPECT().Login(gomock.Any(), reqBody).
					Return(nil, tc.commandsError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}

func (s *AuthHandlerTestSuite) TestLogout() {
	s.Run("success: returns 204 No Content", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, "/auth/logout", nil, "bearer-token")
		s.Equal(http.StatusNoContent, rec.Code)
	})
}

func (s *AuthHandlerTestSuite) TestMe() {
	url := "/auth/me"
	returnUser := builder.NewUserBuilder().BuildReadModel()

	s.Run("success: returns current user info", func() {
		s.mockQueries.EXPECT().GetCurrentUser(gomock.Any(), gomock.Any()).
			Return(returnUser, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "bearer-token")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(returnUser.Email, response["email"])
	})

	s.Run("error: returns 500 when user_id missing in context", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Internal server error")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "user not found",
				commandsError:  queries.ErrUserNotFound,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "User not found",
			},
			{
				name:           "user inactive",
				commandsError:  queries.ErrUserInactive,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Account is inactive",
			},
			{
				name:           "internal server error",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal server error",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockQueries.EXPECT().GetCurrentUser(gomock.Any(), gomock.Any()).
					Return(nil, tc.commandsError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "bearer-token")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}
