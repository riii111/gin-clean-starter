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

type ReviewHandlerTestSuite struct {
	suite.Suite
	router       *gin.Engine
	mockCtrl     *gomock.Controller
	mockCommands *commandsmock.MockReviewCommands
	mockQueries  *queriesmock.MockReviewQueries
	handler      *api.ReviewHandler
}

func (s *ReviewHandlerTestSuite) SetupTest() {
	gin.SetMode(gin.TestMode)
	s.router = gin.New()

	s.mockCtrl = gomock.NewController(s.T())
	s.mockCommands = commandsmock.NewMockReviewCommands(s.mockCtrl)
	s.mockQueries = queriesmock.NewMockReviewQueries(s.mockCtrl)
	s.handler = api.NewReviewHandler(s.mockCommands, s.mockQueries)

	// Mock authentication middleware for testing
	authMiddleware := func(c *gin.Context) {
		if authHeader := c.GetHeader("Authorization"); authHeader != "" {
			// Extract userID from token (mock behavior)
			userID := uuid.New()
			role := user.RoleViewer
			c.Set("user_id", userID)
			c.Set("user_role", role)
		}
		// For unauthenticated requests, don't set any context values
		c.Next()
	}

	// Setup routes
	s.router.POST("/reviews", authMiddleware, s.handler.Create)
	s.router.GET("/reviews/:id", s.handler.Get)
	s.router.PUT("/reviews/:id", authMiddleware, s.handler.Update)
	s.router.DELETE("/reviews/:id", authMiddleware, s.handler.Delete)
	s.router.GET("/resources/:id/reviews", s.handler.ListByResource)
	s.router.GET("/users/:id/reviews", authMiddleware, s.handler.ListByUser)
	s.router.GET("/resources/:id/rating-stats", s.handler.ResourceRatingStats)
}

func (s *ReviewHandlerTestSuite) TearDownTest() {
	s.mockCtrl.Finish()
}

func TestReviewHandlerSuite(t *testing.T) {
	suite.Run(t, new(ReviewHandlerTestSuite))
}

type testCaseReview struct {
	name         string
	mutate       func(m map[string]any)
	expectCode   int
	expectInBody string
}

// ================================================================================
// TestCreate
// ================================================================================

func (s *ReviewHandlerTestSuite) TestCreate() {
	url := "/reviews"

	reqBody := builder.NewReviewBuilder().BuildCreateRequestDTO()
	returnView := builder.NewReviewBuilder().BuildViewQuery()
	expectedResult := &commands.CreateReviewResult{
		ReviewID: returnView.ID,
	}

	// Calculate total successful cases (normal + validation success cases)
	bound := []testCaseReview{
		{name: "Rating境界値OK(1)", mutate: testutil.Field("rating", 1), expectCode: http.StatusCreated},
		{name: "Rating境界値OK(5)", mutate: testutil.Field("rating", 5), expectCode: http.StatusCreated},
		{name: "Rating境界値NG(0)", mutate: testutil.Field("rating", 0), expectCode: http.StatusBadRequest},
		{name: "Rating境界値NG(6)", mutate: testutil.Field("rating", 6), expectCode: http.StatusBadRequest},
		{name: "Comment境界値OK(1000文字)", mutate: testutil.Field("comment", strings.Repeat("a", 1000)), expectCode: http.StatusCreated},
		{name: "Comment境界値NG(1001文字)", mutate: testutil.Field("comment", strings.Repeat("a", 1001)), expectCode: http.StatusBadRequest},
	}

	missing := []testCaseReview{
		{name: "ResourceIDフィールドなし(必須)", mutate: testutil.Field("resourceId", nil), expectCode: http.StatusBadRequest},
		{name: "ReservationIDフィールドなし(必須)", mutate: testutil.Field("reservationId", nil), expectCode: http.StatusBadRequest},
		{name: "Ratingフィールドなし(必須)", mutate: testutil.Field("rating", nil), expectCode: http.StatusBadRequest},
		{name: "Commentフィールドなし(必須)", mutate: testutil.Field("comment", nil), expectCode: http.StatusBadRequest},
	}

	empty := []testCaseReview{
		{name: "Commentが空", mutate: testutil.Field("comment", ""), expectCode: http.StatusBadRequest},
	}

	allValidationTestCases := [][]testCaseReview{bound, missing, empty}

	// Count total successful cases (normal test + validation success cases + use case error tests)
	totalSuccessfulCases := 1 // normal test
	for _, testCaseGroup := range allValidationTestCases {
		for _, tc := range testCaseGroup {
			if tc.expectCode == http.StatusCreated {
				totalSuccessfulCases++
			}
		}
	}

	// Set up mocks for all successful cases at once
	s.mockCommands.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(expectedResult, nil).Times(totalSuccessfulCases)
	s.mockQueries.EXPECT().GetByID(gomock.Any(), expectedResult.ReviewID).
		Return(returnView, nil).Times(totalSuccessfulCases)

	s.Run("正常系: 有効なリクエストで201 Createdが返却される", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "bearer-token")

		var response resdto.ReviewResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusCreated, &response)
		s.Equal(returnView.ID.String(), response.ID)
		s.Equal(returnView.Rating, response.Rating)
		s.Equal(returnView.Comment, response.Comment)
	})

	s.Run("異常系: バリデーションエラーで400 BadRequestが返される", func() {
		// Mock expectations are already set up at the test level

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

					rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, requestMap, "bearer-token")
					if tc.expectCode == http.StatusCreated {
						httptest.AssertSuccessResponse(s.T(), rec, tc.expectCode, nil)
					} else {
						httptest.AssertErrorResponse(s.T(), rec, tc.expectCode, "")
					}
				})
			}
		}
	})

	s.Run("異常系: 認証なしで401 Unauthorizedが返される", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "ドメインバリデーションエラー",
				commandsError:  commands.ErrDomainValidationFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Create review failed",
			},
			{
				name:           "レビュー作成失敗",
				commandsError:  commands.ErrReviewCreationFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Create review failed",
			},
			{
				name:           "内部サーバーエラー",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Create review failed",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockCommands.EXPECT().Create(gomock.Any(), reqBody, gomock.Any()).
					Return(nil, tc.commandsError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "bearer-token")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})

	s.Run("異常系: クエリ失敗で500 Internal Server Errorが返される", func() {
		s.mockCommands.EXPECT().Create(gomock.Any(), reqBody, gomock.Any()).
			Return(expectedResult, nil).Times(1)
		s.mockQueries.EXPECT().GetByID(gomock.Any(), expectedResult.ReviewID).
			Return(nil, errors.New("query failed")).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Failed to load review")
	})
}

// ================================================================================
// TestGet
// ================================================================================

func (s *ReviewHandlerTestSuite) TestGet() {
	reviewID := uuid.New()
	url := "/reviews/" + reviewID.String()

	returnView := builder.NewReviewBuilder().BuildViewQuery()
	returnView.ID = reviewID

	s.Run("正常系: 200 OKでReviewResponseが返却される", func() {
		s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
			Return(returnView, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")

		var response resdto.ReviewResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(reviewID.String(), response.ID)
		s.Equal(returnView.Rating, response.Rating)
		s.Equal(returnView.Comment, response.Comment)
	})

	s.Run("異常系: 無効なUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("異常系: 存在しないレビューで404 Not Foundが返される", func() {
		s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
			Return(nil, queries.ErrReviewNotFound).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusNotFound, "Not found")
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "レビューが見つからない",
				queriesError:   queries.ErrReviewNotFound,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
			{
				name:           "アクセス権限なし",
				queriesError:   queries.ErrReviewAccess,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
			{
				name:           "内部サーバーエラー",
				queriesError:   errors.New("database error"),
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
					Return(nil, tc.queriesError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}

// ================================================================================
// TestUpdate
// ================================================================================

func (s *ReviewHandlerTestSuite) TestUpdate() {
	reviewID := uuid.New()
	url := "/reviews/" + reviewID.String()

	reqBody := builder.NewReviewBuilder().BuildUpdateRequestDTO()
	returnView := builder.NewReviewBuilder().BuildViewQuery()
	returnView.ID = reviewID

	testCases := []testCaseReview{
		{name: "Rating境界値OK(1)", mutate: testutil.Field("rating", 1), expectCode: http.StatusOK},
		{name: "Rating境界値OK(5)", mutate: testutil.Field("rating", 5), expectCode: http.StatusOK},
		{name: "Rating境界値NG(0)", mutate: testutil.Field("rating", 0), expectCode: http.StatusBadRequest},
		{name: "Rating境界値NG(6)", mutate: testutil.Field("rating", 6), expectCode: http.StatusBadRequest},
		{name: "Comment境界値OK(1000文字)", mutate: testutil.Field("comment", strings.Repeat("a", 1000)), expectCode: http.StatusOK},
		{name: "Comment境界値NG(1001文字)", mutate: testutil.Field("comment", strings.Repeat("a", 1001)), expectCode: http.StatusBadRequest},
	}

	totalSuccessfulCases := 1
	for _, tc := range testCases {
		if tc.expectCode == http.StatusOK {
			totalSuccessfulCases++
		}
	}

	s.mockCommands.EXPECT().Update(gomock.Any(), reviewID, gomock.Any(), gomock.Any()).
		Return(nil).Times(totalSuccessfulCases)
	s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
		Return(returnView, nil).Times(totalSuccessfulCases)

	s.Run("正常系: 200 OKで更新後のReviewResponseが返却される", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "bearer-token")

		var response resdto.ReviewResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(reviewID.String(), response.ID)
		s.Equal(returnView.Rating, response.Rating)
		s.Equal(returnView.Comment, response.Comment)
	})

	s.Run("異常系: バリデーションエラーで400 BadRequestが返される", func() {
		for _, tc := range testCases {
			s.Run(tc.name, func() {
				baseMap := func() map[string]any {
					bytes, _ := json.Marshal(reqBody)
					var m map[string]any
					_ = json.Unmarshal(bytes, &m)
					return m
				}()
				requestMap := maps.Clone(baseMap)
				tc.mutate(requestMap)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, requestMap, "bearer-token")
				if tc.expectCode == http.StatusOK {
					httptest.AssertSuccessResponse(s.T(), rec, tc.expectCode, nil)
				} else {
					httptest.AssertErrorResponse(s.T(), rec, tc.expectCode, "")
				}
			})
		}
	})

	s.Run("異常系: 無効なUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, invalidURL, reqBody, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("異常系: 認証なしで401 Unauthorizedが返される", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "レビューが所有者でない",
				commandsError:  commands.ErrReviewNotOwned,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Update failed",
			},
			{
				name:           "レビューが見つからない",
				commandsError:  commands.ErrReviewNotFoundWrite,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Update failed",
			},
			{
				name:           "レビュー更新失敗",
				commandsError:  commands.ErrReviewUpdateFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Update failed",
			},
			{
				name:           "内部サーバーエラー",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Update failed",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockCommands.EXPECT().Update(gomock.Any(), reviewID, reqBody, gomock.Any()).
					Return(tc.commandsError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "bearer-token")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})

	s.Run("異常系: クエリ失敗で500 Internal Server Errorが返される", func() {
		s.mockCommands.EXPECT().Update(gomock.Any(), reviewID, reqBody, gomock.Any()).
			Return(nil).Times(1)
		s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
			Return(nil, errors.New("query failed")).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Failed to load review")
	})
}

// ================================================================================
// TestDelete
// ================================================================================

func (s *ReviewHandlerTestSuite) TestDelete() {
	reviewID := uuid.New()
	url := "/reviews/" + reviewID.String()

	s.Run("正常系: 204 No Contentが返却される", func() {
		s.mockCommands.EXPECT().Delete(gomock.Any(), reviewID, gomock.Any(), string(user.RoleViewer)).
			Return(nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, url, nil, "bearer-token")
		s.Equal(http.StatusNoContent, rec.Code)
	})

	s.Run("異常系: 無効なUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, invalidURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("異常系: 認証なしで401 Unauthorizedが返される", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, url, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "レビューが所有者でない",
				commandsError:  commands.ErrReviewNotOwned,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Delete failed",
			},
			{
				name:           "レビューが見つからない",
				commandsError:  commands.ErrReviewNotFoundWrite,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Delete failed",
			},
			{
				name:           "レビュー削除失敗",
				commandsError:  commands.ErrReviewDeletionFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Delete failed",
			},
			{
				name:           "内部サーバーエラー",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Delete failed",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockCommands.EXPECT().Delete(gomock.Any(), reviewID, gomock.Any(), string(user.RoleViewer)).
					Return(tc.commandsError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, url, nil, "bearer-token")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})

	s.Run("異常系: 管理者権限での削除テスト", func() {
		// Setup admin auth middleware
		adminRouter := gin.New()
		adminAuthMiddleware := func(c *gin.Context) {
			if authHeader := c.GetHeader("Authorization"); authHeader != "" {
				userID := uuid.New()
				role := user.RoleAdmin
				c.Set("user_id", userID)
				c.Set("user_role", role)
			}
			c.Next()
		}
		adminRouter.DELETE("/reviews/:id", adminAuthMiddleware, s.handler.Delete)

		s.mockCommands.EXPECT().Delete(gomock.Any(), reviewID, gomock.Any(), string(user.RoleAdmin)).
			Return(nil).Times(1)

		rec := httptest.PerformRequest(s.T(), adminRouter, http.MethodDelete, url, nil, "bearer-token")
		s.Equal(http.StatusNoContent, rec.Code)
	})
}

// ================================================================================
// TestListByResource
// ================================================================================

func (s *ReviewHandlerTestSuite) TestListByResource() {
	resourceID := uuid.New()
	baseURL := "/resources/" + resourceID.String() + "/reviews"

	items := []*queries.ReviewListItem{
		builder.NewReviewBuilder().WithRating(5).BuildListItem(),
		builder.NewReviewBuilder().WithRating(4).BuildListItem(),
		builder.NewReviewBuilder().WithRating(3).BuildListItem(),
	}

	s.Run("正常系: リソースのレビューリストが返却される", func() {
		expectedFilters := queries.ReviewFilters{}
		s.mockQueries.EXPECT().ListByResource(gomock.Any(), resourceID, expectedFilters, (*queries.Cursor)(nil), 20).
			Return(items, nil, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		reviews, ok := response["reviews"].([]any)
		s.True(ok)
		s.Equal(len(items), len(reviews))
	})

	s.Run("正常系: ページネーションとフィルタが動作する", func() {
		url := baseURL + "?min_rating=4&max_rating=5&limit=10&after=cursor123"
		minRating := 4
		maxRating := 5
		expectedFilters := queries.ReviewFilters{MinRating: &minRating, MaxRating: &maxRating}
		expectedCursor := &queries.Cursor{After: "cursor123"}
		nextCursor := &queries.Cursor{After: "next_cursor456"}

		s.mockQueries.EXPECT().ListByResource(gomock.Any(), resourceID, expectedFilters, expectedCursor, 10).
			Return(items[:2], nextCursor, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		reviews, ok := response["reviews"].([]any)
		s.True(ok)
		s.Equal(2, len(reviews))
		s.Equal("next_cursor456", response["next_cursor"])
	})

	s.Run("異常系: 無効なリソースUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/resources/invalid-uuid/reviews"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid resource id")
	})

	s.Run("異常系: クエリエラーで500 Internal Server Errorが返される", func() {
		expectedFilters := queries.ReviewFilters{}
		s.mockQueries.EXPECT().ListByResource(gomock.Any(), resourceID, expectedFilters, (*queries.Cursor)(nil), 20).
			Return(nil, nil, errors.New("database error")).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Internal error")
	})

	s.Run("正常系: フィルタパラメータの境界値テスト", func() {
		testCases := []struct {
			name      string
			params    string
			minRating *int
			maxRating *int
		}{
			{
				name:      "min_rating=1",
				params:    "?min_rating=1",
				minRating: func() *int { v := 1; return &v }(),
				maxRating: nil,
			},
			{
				name:      "max_rating=5",
				params:    "?max_rating=5",
				minRating: nil,
				maxRating: func() *int { v := 5; return &v }(),
			},
			{
				name:      "min_rating=1&max_rating=5",
				params:    "?min_rating=1&max_rating=5",
				minRating: func() *int { v := 1; return &v }(),
				maxRating: func() *int { v := 5; return &v }(),
			},
			{
				name:      "無効なmin_rating(文字列)は無視される",
				params:    "?min_rating=invalid",
				minRating: nil,
				maxRating: nil,
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				url := baseURL + tc.params
				expectedFilters := queries.ReviewFilters{MinRating: tc.minRating, MaxRating: tc.maxRating}

				s.mockQueries.EXPECT().ListByResource(gomock.Any(), resourceID, expectedFilters, (*queries.Cursor)(nil), 20).
					Return(items, nil, nil).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
				httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, nil)
			})
		}
	})
}

// ================================================================================
// TestListByUser
// ================================================================================

func (s *ReviewHandlerTestSuite) TestListByUser() {
	userID := uuid.New()
	baseURL := "/users/" + userID.String() + "/reviews"

	items := []*queries.ReviewListItem{
		builder.NewReviewBuilder().WithUserID(userID).BuildListItem(),
		builder.NewReviewBuilder().WithUserID(userID).BuildListItem(),
	}

	s.Run("正常系: ユーザーのレビューリストが返却される", func() {
		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), (*queries.Cursor)(nil), 20).
			Return(items, nil, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "bearer-token")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		reviews, ok := response["reviews"].([]any)
		s.True(ok)
		s.Equal(len(items), len(reviews))
	})

	s.Run("正常系: ページネーションが動作する", func() {
		url := baseURL + "?limit=10&after=cursor123"
		expectedCursor := &queries.Cursor{After: "cursor123"}
		nextCursor := &queries.Cursor{After: "next_cursor456"}

		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), expectedCursor, 10).
			Return(items[:1], nextCursor, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "bearer-token")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		reviews, ok := response["reviews"].([]any)
		s.True(ok)
		s.Equal(1, len(reviews))
		s.Equal("next_cursor456", response["next_cursor"])
	})

	s.Run("異常系: 無効なユーザーUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/users/invalid-uuid/reviews"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid user id")
	})

	s.Run("異常系: 認証なし（匿名ユーザー）でアクセスした場合", func() {
		// Setup router without authentication for this test
		noAuthRouter := gin.New()
		noAuthRouter.GET("/users/:id/reviews", func(c *gin.Context) {
			// No authentication middleware
			s.handler.ListByUser(c)
		})

		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, uuid.Nil, "", (*queries.Cursor)(nil), 20).
			Return(items, nil, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), noAuthRouter, http.MethodGet, baseURL, nil, "")
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, nil)
	})

	s.Run("異常系: アクセス権限エラーで403 Forbiddenが返される", func() {
		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), (*queries.Cursor)(nil), 20).
			Return(nil, nil, queries.ErrReviewAccess).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusForbidden, "Access denied")
	})

	s.Run("正常系: 管理者権限でのアクセステスト", func() {
		// Setup admin auth middleware
		adminRouter := gin.New()
		adminAuthMiddleware := func(c *gin.Context) {
			if authHeader := c.GetHeader("Authorization"); authHeader != "" {
				actorID := uuid.New()
				role := user.RoleAdmin
				c.Set("user_id", actorID)
				c.Set("user_role", role)
			}
			c.Next()
		}
		adminRouter.GET("/users/:id/reviews", adminAuthMiddleware, s.handler.ListByUser)

		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleAdmin), (*queries.Cursor)(nil), 20).
			Return(items, nil, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), adminRouter, http.MethodGet, baseURL, nil, "bearer-token")
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, nil)
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "アクセス権限なし",
				queriesError:   queries.ErrReviewAccess,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Access denied",
			},
			{
				name:           "クエリ失敗",
				queriesError:   queries.ErrReviewQueryFailed,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Access denied",
			},
			{
				name:           "内部サーバーエラー",
				queriesError:   errors.New("database error"),
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Access denied",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), (*queries.Cursor)(nil), 20).
					Return(nil, nil, tc.queriesError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "bearer-token")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}

// ================================================================================
// TestResourceRatingStats
// ================================================================================

func (s *ReviewHandlerTestSuite) TestResourceRatingStats() {
	resourceID := uuid.New()
	url := "/resources/" + resourceID.String() + "/rating-stats"

	expectedStats := builder.NewReviewBuilder().WithResourceID(resourceID).BuildResourceRatingStats()

	s.Run("正常系: 200 OKでResourceRatingStatsResponseが返却される", func() {
		s.mockQueries.EXPECT().GetResourceRatingStats(gomock.Any(), resourceID).
			Return(expectedStats, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")

		var response resdto.ResourceRatingStatsResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(resourceID.String(), response.ResourceID)
		s.Equal(expectedStats.TotalReviews, response.TotalReviews)
		s.Equal(expectedStats.AverageRating, response.AverageRating)
		s.Equal(expectedStats.Rating1Count, response.Rating1Count)
		s.Equal(expectedStats.Rating5Count, response.Rating5Count)
	})

	s.Run("異常系: 無効なリソースUUIDで400 Bad Requestが返される", func() {
		invalidURL := "/resources/invalid-uuid/rating-stats"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid resource id")
	})

	s.Run("異常系: ユースケース起因のエラーの場合、適切なステータスコードが返却される", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "統計が見つからない",
				queriesError:   queries.ErrReviewNotFound,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Failed to get stats",
			},
			{
				name:           "クエリ失敗",
				queriesError:   queries.ErrReviewQueryFailed,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Failed to get stats",
			},
			{
				name:           "内部サーバーエラー",
				queriesError:   errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Failed to get stats",
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				s.mockQueries.EXPECT().GetResourceRatingStats(gomock.Any(), resourceID).
					Return(nil, tc.queriesError).Times(1)

				rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
				httptest.AssertErrorResponse(s.T(), rec, tc.expectedStatus, tc.expectedMsg)
			})
		}
	})
}
