//go:build unit

package api_test

import (
	"errors"
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
		if c.GetHeader("Authorization") == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"message": "Unauthorized"}})
			return
		}
		// Mock authenticated user
		c.Set("user_id", uuid.New())
		c.Set("user_role", user.RoleViewer)
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
	expectedResult := &commands.CreateReviewResult{ReviewID: returnView.ID}

	// Validation boundary cases
	bound := []testCaseReview{
		{name: "rating boundary OK (1)", mutate: testutil.Field("rating", 1), expectCode: http.StatusCreated},
		{name: "rating boundary OK (5)", mutate: testutil.Field("rating", 5), expectCode: http.StatusCreated},
		{name: "rating boundary invalid (0)", mutate: testutil.Field("rating", 0), expectCode: http.StatusBadRequest},
		{name: "rating boundary invalid (6)", mutate: testutil.Field("rating", 6), expectCode: http.StatusBadRequest},
		{name: "comment length OK (1000 chars)", mutate: testutil.Field("comment", strings.Repeat("a", 1000)), expectCode: http.StatusCreated},
		{name: "comment length invalid (1001 chars)", mutate: testutil.Field("comment", strings.Repeat("a", 1001)), expectCode: http.StatusBadRequest},
	}

	missing := []testCaseReview{
		{name: "missing field: resourceId (required)", mutate: testutil.Field("resourceId", nil), expectCode: http.StatusBadRequest},
		{name: "missing field: reservationId (required)", mutate: testutil.Field("reservationId", nil), expectCode: http.StatusBadRequest},
		{name: "missing field: rating (required)", mutate: testutil.Field("rating", nil), expectCode: http.StatusBadRequest},
		{name: "missing field: comment (required)", mutate: testutil.Field("comment", nil), expectCode: http.StatusBadRequest},
	}

	empty := []testCaseReview{
		{name: "empty comment", mutate: testutil.Field("comment", ""), expectCode: http.StatusBadRequest},
	}

	allValidationTestCases := [][]testCaseReview{bound, missing, empty}

	s.Run("success: returns 201 Created for valid request", func() {
		s.mockCommands.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(expectedResult, nil).Times(1)
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "bearer-token")

		var body map[string]string
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusCreated, &body)
		s.Equal(returnView.ID.String(), body["id"])
		httptest.AssertHeaders(s.T(), rec, map[string]string{"Location": "/reviews/" + returnView.ID.String()})
	})

	s.Run("error: 400 Bad Request on validation errors", func() {
		// Mock expectations are already set up at the test level

		for _, testCaseGroup := range allValidationTestCases {
			for _, tc := range testCaseGroup {
				s.Run(tc.name, func() {
					requestMap := testutil.DtoMap(s.T(), reqBody, tc.mutate)

					if tc.expectCode == http.StatusCreated {
						s.mockCommands.EXPECT().Create(gomock.Any(), gomock.Any(), gomock.Any()).
							Return(expectedResult, nil).Times(1)
					}
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

	s.Run("error: 401 Unauthorized when unauthenticated", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPost, url, reqBody, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "domain validation error",
				commandsError:  commands.ErrDomainValidationFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Invalid request",
			},
			{
				name:           "review creation failed",
				commandsError:  commands.ErrReviewCreationFailed,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
			},
			{
				name:           "internal server error",
				commandsError:  errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
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

	// Query is no longer called in Create; skip query failure case.
}

// ================================================================================
// TestGet
// ================================================================================

func (s *ReviewHandlerTestSuite) TestGet() {
	reviewID := uuid.New()
	url := "/reviews/" + reviewID.String()

	returnView := builder.NewReviewBuilder().BuildViewQuery()
	returnView.ID = reviewID

	s.Run("success: returns 200 OK with ReviewResponse", func() {
		s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
			Return(returnView, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")

		var response resdto.ReviewResponse
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		s.Equal(reviewID.String(), response.ID)
		s.Equal(returnView.Rating, response.Rating)
		s.Equal(returnView.Comment, response.Comment)
	})

	s.Run("error: 400 Bad Request for invalid UUID", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("error: 404 Not Found for missing review", func() {
		s.mockQueries.EXPECT().GetByID(gomock.Any(), reviewID).
			Return(nil, queries.ErrReviewNotFound).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, url, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusNotFound, "Not found")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "review not found",
				queriesError:   queries.ErrReviewNotFound,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
			{
				name:           "query failed",
				queriesError:   queries.ErrReviewQueryFailed,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
			},
			{
				name:           "internal server error",
				queriesError:   errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
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
		{name: "rating boundary OK (1)", mutate: testutil.Field("rating", 1), expectCode: http.StatusNoContent},
		{name: "rating boundary OK (5)", mutate: testutil.Field("rating", 5), expectCode: http.StatusNoContent},
		{name: "rating boundary invalid (0)", mutate: testutil.Field("rating", 0), expectCode: http.StatusBadRequest},
		{name: "rating boundary invalid (6)", mutate: testutil.Field("rating", 6), expectCode: http.StatusBadRequest},
		{name: "comment length OK (1000 chars)", mutate: testutil.Field("comment", strings.Repeat("a", 1000)), expectCode: http.StatusNoContent},
		{name: "comment length invalid (1001 chars)", mutate: testutil.Field("comment", strings.Repeat("a", 1001)), expectCode: http.StatusBadRequest},
	}

	s.Run("success: returns 204 No Content", func() {
		s.mockCommands.EXPECT().Update(gomock.Any(), reviewID, gomock.Any(), gomock.Any()).
			Return(nil).Times(1)
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "bearer-token")
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusNoContent, nil)
	})

	s.Run("error: 400 Bad Request on validation errors", func() {
		for _, tc := range testCases {
			s.Run(tc.name, func() {
				requestMap := testutil.DtoMap(s.T(), reqBody, tc.mutate)

				if tc.expectCode == http.StatusNoContent {
					s.mockCommands.EXPECT().Update(gomock.Any(), reviewID, gomock.Any(), gomock.Any()).
						Return(nil).Times(1)
				}
				rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, requestMap, "bearer-token")
				if tc.expectCode == http.StatusNoContent {
					httptest.AssertSuccessResponse(s.T(), rec, tc.expectCode, nil)
				} else {
					httptest.AssertErrorResponse(s.T(), rec, tc.expectCode, "")
				}
			})
		}
	})

	s.Run("error: 400 Bad Request for invalid UUID", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, invalidURL, reqBody, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("error: 401 Unauthorized when unauthenticated", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodPut, url, reqBody, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "review not owned",
				commandsError:  commands.ErrReviewNotOwned,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Forbidden",
			},
			{
				name:           "review not found",
				commandsError:  commands.ErrReviewNotFoundWrite,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
			{
				name:           "review update failed",
				commandsError:  commands.ErrReviewUpdateFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Update failed",
			},
			{
				name:           "internal server error",
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

}

// ================================================================================
// TestDelete
// ================================================================================

func (s *ReviewHandlerTestSuite) TestDelete() {
	reviewID := uuid.New()
	url := "/reviews/" + reviewID.String()

	s.Run("success: returns 204 No Content", func() {
		s.mockCommands.EXPECT().Delete(gomock.Any(), reviewID, gomock.Any(), string(user.RoleViewer)).
			Return(nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, url, nil, "bearer-token")
		s.Equal(http.StatusNoContent, rec.Code)
	})

	s.Run("error: 400 Bad Request for invalid UUID", func() {
		invalidURL := "/reviews/invalid-uuid"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, invalidURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid id")
	})

	s.Run("error: 401 Unauthorized when unauthenticated", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodDelete, url, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			commandsError  error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "review not owned",
				commandsError:  commands.ErrReviewNotOwned,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Forbidden",
			},
			{
				name:           "review not found",
				commandsError:  commands.ErrReviewNotFoundWrite,
				expectedStatus: http.StatusNotFound,
				expectedMsg:    "Not found",
			},
			{
				name:           "review delete failed",
				commandsError:  commands.ErrReviewDeletionFailed,
				expectedStatus: http.StatusBadRequest,
				expectedMsg:    "Delete failed",
			},
			{
				name:           "internal server error",
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

	s.Run("success: delete as admin", func() {
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

	s.Run("success: returns review list by resource", func() {
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

	s.Run("success: pagination and filters work", func() {
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

	s.Run("error: 400 Bad Request for invalid resource UUID", func() {
		invalidURL := "/resources/invalid-uuid/reviews"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid resource id")
	})

	s.Run("error: returns 500 Internal Server Error on query error", func() {
		expectedFilters := queries.ReviewFilters{}
		s.mockQueries.EXPECT().ListByResource(gomock.Any(), resourceID, expectedFilters, (*queries.Cursor)(nil), 20).
			Return(nil, nil, errors.New("database error")).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusInternalServerError, "Internal error")
	})

	s.Run("success: filter parameter boundary tests", func() {
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
				name:      "ignores invalid min_rating (string)",
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

	s.Run("success: returns review list by user", func() {
		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), (*queries.Cursor)(nil), 20).
			Return(items, nil, nil).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "bearer-token")

		var response map[string]any
		httptest.AssertSuccessResponse(s.T(), rec, http.StatusOK, &response)
		reviews, ok := response["reviews"].([]any)
		s.True(ok)
		s.Equal(len(items), len(reviews))
	})

	s.Run("success: pagination works", func() {
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

	s.Run("error: 400 Bad Request for invalid user UUID", func() {
		invalidURL := "/users/invalid-uuid/reviews"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid user id")
	})

	s.Run("error: 401 Unauthorized when unauthenticated (anonymous)", func() {
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusUnauthorized, "Unauthorized")
	})

	s.Run("error: 403 Forbidden on access denied", func() {
		s.mockQueries.EXPECT().ListByUser(gomock.Any(), userID, gomock.Any(), string(user.RoleViewer), (*queries.Cursor)(nil), 20).
			Return(nil, nil, queries.ErrReviewAccess).Times(1)

		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, baseURL, nil, "bearer-token")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusForbidden, "Access denied")
	})

	s.Run("success: access with admin role", func() {
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

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "access denied",
				queriesError:   queries.ErrReviewAccess,
				expectedStatus: http.StatusForbidden,
				expectedMsg:    "Access denied",
			},
			{
				name:           "query failed",
				queriesError:   queries.ErrReviewQueryFailed,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
			},
			{
				name:           "internal server error",
				queriesError:   errors.New("database error"),
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Internal error",
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

	s.Run("success: returns 200 OK with ResourceRatingStatsResponse", func() {
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

	s.Run("error: 400 Bad Request for invalid resource UUID", func() {
		invalidURL := "/resources/invalid-uuid/rating-stats"
		rec := httptest.PerformRequest(s.T(), s.router, http.MethodGet, invalidURL, nil, "")
		httptest.AssertErrorResponse(s.T(), rec, http.StatusBadRequest, "Invalid resource id")
	})

	s.Run("error: maps usecase errors to proper statuses", func() {
		testCases := []struct {
			name           string
			queriesError   error
			expectedStatus int
			expectedMsg    string
		}{
			{
				name:           "stats not found",
				queriesError:   queries.ErrReviewNotFound,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Failed to get stats",
			},
			{
				name:           "query failed",
				queriesError:   queries.ErrReviewQueryFailed,
				expectedStatus: http.StatusInternalServerError,
				expectedMsg:    "Failed to get stats",
			},
			{
				name:           "internal server error",
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
