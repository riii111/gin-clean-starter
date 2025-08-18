//go:build unit

package api_test

import (
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"strings"
	"testing"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/handler/api"
	resdto "gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/internal/pkg/config"
	"gin-clean-starter/internal/pkg/jwt"
	"gin-clean-starter/internal/usecase"
	"gin-clean-starter/tests/common/builder"
	"gin-clean-starter/tests/common/helper"
	usecasemock "gin-clean-starter/tests/mock/usecase"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"
)

type AuthHandlerTestSuite struct {
	suite.Suite
	router   *gin.Engine
	mockCtrl *gomock.Controller
	mockAUC  *usecasemock.MockAuthUseCase
	handler  *api.AuthHandler
}

func (s *AuthHandlerTestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	s.mockCtrl = gomock.NewController(s.T())
	s.mockAUC = usecasemock.NewMockAuthUseCase(s.mockCtrl)
	mockJWTService := &jwt.Service{} // Mock JWT service for testing
	s.handler = api.NewAuthHandler(s.mockAUC, mockJWTService, config.NewTestConfig())

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

	s.Run("正常系: 有効なリクエストで200 OKが返却される", func() {
		creds, _ := user.NewCredentials(reqBody.Email, reqBody.Password)
		s.mockAUC.EXPECT().Login(gomock.Any(), creds).
			Return(&usecase.TokenPair{AccessToken: expectedToken, RefreshToken: expectedRefresh}, returnUser, nil).Times(1)
		rec := helper.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")

		var response resdto.LoginResponse
		helper.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(returnUser.Email, response.User.Email)
	})

	s.Run("異常系: バリデーションエラーで400 BadRequestが返される", func() {
		bound := []testCaseAuth{
			{name: "Email境界値OK(有効なメール)", mutate: helper.Field("email", "valid@example.com"), expectCode: http.StatusOK},
			{name: "Email境界値NG(無効なメール)", mutate: helper.Field("email", "invalid-email"), expectCode: http.StatusBadRequest},
			{name: "Password境界値OK(8文字)", mutate: helper.Field("password", "password"), expectCode: http.StatusOK},
			{name: "Password境界値NG(7文字)", mutate: helper.Field("password", strings.Repeat("a", 7)), expectCode: http.StatusBadRequest},
		}

		missing := []testCaseAuth{
			{name: "Emailフィールドなし(必須)", mutate: helper.Field("email", nil), expectCode: http.StatusBadRequest},
			{name: "Passwordフィールドなし(必須)", mutate: helper.Field("password", nil), expectCode: http.StatusBadRequest},
		}

		empty := []testCaseAuth{
			{name: "Emailが空", mutate: helper.Field("email", ""), expectCode: http.StatusBadRequest},
			{name: "Passwordが空", mutate: helper.Field("password", ""), expectCode: http.StatusBadRequest},
		}

		allValidationTestCases := [][]testCaseAuth{bound, missing, empty}

		for _, testCaseGroup := range allValidationTestCases {
			for _, tc := range testCaseGroup {
				s.Run(tc.name, func() {
					baseMap := func() map[string]any {
						bytes, _ := json.Marshal(reqBody)
						var m map[string]any
						_ = json.Unmarshal(bytes, &m)
						return m
					}()
					requestMap := maps.Clone(baseMap)
					tc.mutate(requestMap)

					if tc.expectCode == http.StatusOK {
						email, _ := requestMap["email"].(string)
						password, _ := requestMap["password"].(string)
						creds, _ := user.NewCredentials(email, password)
						s.mockAUC.EXPECT().Login(gomock.Any(), creds).
							Return(&usecase.TokenPair{AccessToken: expectedToken, RefreshToken: expectedRefresh}, returnUser, nil)
					}
					rec := helper.PerformRequest(s.T(), s.router, http.MethodPost, url, requestMap, "")
					if tc.expectCode == http.StatusOK {
						helper.AssertSuccessResponse(s.T(), rec, tc.expectCode, nil)
					} else {
						helper.AssertErrorResponse(s.T(), rec, tc.expectCode, "")
					}
				})
			}
		}
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			usecaseError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "認証失敗",
				usecaseError:   usecase.ErrInvalidCredentials,
				expectedStatus: http.StatusUnauthorized,
				expectedMsg:    "Invalid email or password",
			},
			{
				name:           "ユーザー見つからない",
				usecaseError:   usecase.ErrUserNotFound,
				expectedStatus: http.StatusUnauthorized,
				expectedMsg:    "Invalid email or password",
			},
			{
				name:           "ユーザー無効",
				usecaseError:   usecase.ErrUserInactive,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Account is inactive",
			},
			{
				name:           "内部サーバーエラー",
				usecaseError:   errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal server error",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				creds, _ := user.NewCredentials(reqBody.Email, reqBody.Password)
				s.mockAUC.EXPECT().Login(gomock.Any(), creds).
					Return(nil, nil, tc.usecaseError).Times(1)

				rec := helper.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")
				helper.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}

func (s *AuthHandlerTestSuite) TestLogout() {
	s.Run("正常系: 204 No Contentが返却される", func() {
		rec := helper.PerformRequest(s.T(), s.router, http.MethodPost, "/auth/logout", nil, "bearer-token")
		s.Equal(http.StatusNoContent, rec.Code)
	})
}

func (s *AuthHandlerTestSuite) TestMe() {
	url := "/auth/me"
	returnUser := builder.NewUserBuilder().BuildReadModel()

	s.Run("正常系: 認証済みユーザー情報が返却される", func() {
		s.mockAUC.EXPECT().GetCurrentUser(gomock.Any(), gomock.Any()).
			Return(returnUser, nil).Times(1)

		rec := helper.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "bearer-token")

		var response map[string]any
		helper.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(returnUser.Email, response["email"])
	})

	s.Run("異常系: 認証なしで500が返却される", func() {
		rec := helper.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
		helper.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Internal server error")
	})
}
