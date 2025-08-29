//go:build e2e

package review_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/handler/dto/request"
	"gin-clean-starter/internal/handler/dto/response"
	"gin-clean-starter/tests/common/authtest"
	"gin-clean-starter/tests/common/builder"
	"gin-clean-starter/tests/common/dbtest"
	"gin-clean-starter/tests/common/httptest"
	"gin-clean-starter/tests/e2e"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

const (
	reviewsURL         = "/api/reviews"
	resourceReviewsURL = "/api/resources/%s/reviews"
	userReviewsURL     = "/api/users/%s/reviews"
	ratingStatsURL     = "/api/resources/%s/rating-stats"
)

type ReviewSuite struct {
	e2e.SharedSuite
}

func (s *ReviewSuite) SetupSubTest() {
	s.SharedSuite.SetupSubTest()
}

func TestReviewSuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(ReviewSuite))
}

// =============================================================================
// TestCreateReview - Review creation API tests
// =============================================================================

func (s *ReviewSuite) TestCreateReview() {
	url := reviewsURL

	s.Run("Normal case: User can create review successfully", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer@example.com", "password123")

		reqBody := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(5).
			WithComment("Excellent service!").
			BuildCreateRequestDTO()

		w := httptest.PerformRequest(t, s.Router, http.MethodPost, url, reqBody, token)
		require.Equal(t, http.StatusCreated, w.Code, "Should create review successfully")

		var created map[string]string
		err := httptest.DecodeResponseBody(t, w.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id, "Review ID should not be empty")
		// Location header format may vary by router prefix; only assert body id

		// Fetch detail and assert
		detailURL := reviewsURL + "/" + id
		dw := httptest.PerformRequest(t, s.Router, http.MethodGet, detailURL, nil, "")
		require.Equal(t, http.StatusOK, dw.Code)

		var actualRes response.ReviewResponse
		err = httptest.DecodeResponseBody(t, dw.Body, &actualRes)
		require.NoError(t, err)

		expected := &response.ReviewResponse{
			UserEmail:    "reviewer@example.com",
			ResourceName: "Test Resource",
			Rating:       int32(5),
			Comment:      "Excellent service!",
		}

		opts := []cmp.Option{
			cmpopts.IgnoreFields(response.ReviewResponse{}, "ID", "UserID", "ResourceID", "ReservationID", "CreatedAt", "UpdatedAt"),
		}

		if diff := cmp.Diff(expected, &actualRes, opts...); diff != "" {
			t.Errorf("Review response mismatch (-want +got):\n%s", diff)
		}
	})

	s.Run("Error case: Duplicate review for same reservation fails", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer2@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource 2", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer2@example.com", "password123")

		reqBody := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(4).
			WithComment("First review").
			BuildCreateRequestDTO()

		// First review creation
		w1 := httptest.PerformRequest(t, s.Router, http.MethodPost, url, reqBody, token)
		require.Equal(t, http.StatusCreated, w1.Code)

		// Second review attempt with same reservation
		reqBody.Comment = "Second review attempt"
		w2 := httptest.PerformRequest(t, s.Router, http.MethodPost, url, reqBody, token)
		require.Equal(t, http.StatusConflict, w2.Code, "Should prevent duplicate reviews for same reservation")
	})

	s.Run("Auth test - Unauthorized when not logged in", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer4@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource 4", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		reqBody := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(5).
			WithComment("Great service!").
			BuildCreateRequestDTO()

		w := httptest.PerformRequest(t, s.Router, http.MethodPost, url, reqBody, "")
		require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject unauthorized access")
	})
}

// =============================================================================
// TestGetReview - Review detail retrieval API tests
// =============================================================================

func (s *ReviewSuite) TestGetReview() {
	s.Run("Normal case: Review retrieved successfully by ID", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(4).
			WithComment("Good service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Get the review
		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "Should retrieve review successfully")

		var actualRes response.ReviewResponse
		err = httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)
		require.Equal(t, id, actualRes.ID)

		expected := &response.ReviewResponse{
			Rating:  int32(4),
			Comment: "Good service",
		}

		opts := []cmp.Option{
			cmpopts.IgnoreFields(response.ReviewResponse{}, "UserID", "UserEmail", "ResourceID", "ResourceName", "ReservationID", "CreatedAt", "UpdatedAt"),
		}

		expected.ID = id

		if diff := cmp.Diff(expected, &actualRes, opts...); diff != "" {
			t.Errorf("Review response mismatch (-want +got):\n%s", diff)
		}
	})

	s.Run("Error case: Returns 404 Not Found for non-existent ID", func() {
		t := s.T()

		nonExistentID := uuid.New().String()
		url := reviewsURL + "/" + nonExistentID
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusNotFound, w.Code, "Should return not found for non-existent review")
	})
}

// =============================================================================
// TestUpdateReview - Review update API tests
// =============================================================================

func (s *ReviewSuite) TestUpdateReview() {
	s.Run("Normal case: User can update their own review", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(3).
			WithComment("Average service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Update the review
		rating := 5
		comment := "Excellent updated service!"
		updateReq := request.UpdateReviewRequest{
			Rating:  &rating,
			Comment: &comment,
		}

		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodPut, url, updateReq, token)
		require.Equal(t, http.StatusNoContent, w.Code, "Should update review successfully")

		// Fetch updated review to verify changes
		getResp := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, token)
		require.Equal(t, http.StatusOK, getResp.Code)
		var updatedReview response.ReviewResponse
		err = httptest.DecodeResponseBody(t, getResp.Body, &updatedReview)
		require.NoError(t, err)
		require.Equal(t, id, updatedReview.ID)
		require.Equal(t, int32(5), updatedReview.Rating)
		require.Equal(t, "Excellent updated service!", updatedReview.Comment)
	})

	s.Run("Normal case: Partial update (rating only)", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer2@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource 2", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer2@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(3).
			WithComment("Average service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Update only rating
		rating := 4
		updateReq := request.UpdateReviewRequest{
			Rating: &rating,
		}

		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodPut, url, updateReq, token)
		require.Equal(t, http.StatusNoContent, w.Code, "Should update rating only")

		// Fetch updated review to verify changes
		getResp := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, token)
		require.Equal(t, http.StatusOK, getResp.Code)
		var updatedReview response.ReviewResponse
		err = httptest.DecodeResponseBody(t, getResp.Body, &updatedReview)
		require.NoError(t, err)
		require.Equal(t, id, updatedReview.ID)
		require.Equal(t, int32(4), updatedReview.Rating)
		require.Equal(t, "Average service", updatedReview.Comment) // Comment unchanged
	})

	s.Run("Auth test - Unauthorized when not logged in", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer3@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource 3", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer3@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(3).
			WithComment("Average service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Try to update without authentication
		rating := 5
		updateReq := request.UpdateReviewRequest{
			Rating: &rating,
		}

		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodPut, url, updateReq, "")
		require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject unauthorized access")
	})
}

// =============================================================================
// TestDeleteReview - Review deletion API tests
// =============================================================================

func (s *ReviewSuite) TestDeleteReview() {
	s.Run("Normal case: User can delete their own review", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(2).
			WithComment("Poor service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Delete the review
		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodDelete, url, nil, token)
		require.Equal(t, http.StatusNoContent, w.Code, "Should delete review successfully")

		// Verify the review is deleted
		getResp := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusNotFound, getResp.Code, "Review should be deleted")
	})

	s.Run("Normal case: Admin can delete other users' reviews", func() {
		t := s.T()

		// Create regular user and their review
		regularUserID := dbtest.CreateTestUser(t, s.DB, "regular@example.com", string(user.RoleViewer))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, regularUserID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		regularToken := authtest.LoginUser(t, s.Router, "regular@example.com", "password123")

		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(1).
			WithComment("Very poor").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, regularToken)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Admin tries to delete the regular user's review
		dbtest.CreateTestUser(t, s.DB, "admin@example.com", string(user.RoleAdmin))
		adminToken := authtest.LoginUser(t, s.Router, "admin@example.com", "password123")

		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodDelete, url, nil, adminToken)
		require.Equal(t, http.StatusNoContent, w.Code, "Admin should be able to delete any review")
	})

	s.Run("Auth test - Unauthorized when not logged in", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "reviewer2@example.com", string(user.RoleAdmin))
		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource 2", 60)

		now := time.Now()
		reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "reviewer2@example.com", "password123")

		// Create a review first
		createReq := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservationID).
			WithRating(2).
			WithComment("Poor service").
			BuildCreateRequestDTO()

		createResp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, createReq, token)
		require.Equal(t, http.StatusCreated, createResp.Code)

		var created map[string]string
		err := httptest.DecodeResponseBody(t, createResp.Body, &created)
		require.NoError(t, err)
		id := created["id"]
		require.NotEmpty(t, id)

		// Try to delete without authentication
		url := reviewsURL + "/" + id
		w := httptest.PerformRequest(t, s.Router, http.MethodDelete, url, nil, "")
		require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject unauthorized access")
	})
}

// =============================================================================
// TestListResourceReviews - Resource reviews list API tests
// =============================================================================

func (s *ReviewSuite) TestListResourceReviews() {
	s.Run("Normal case: Reviews list retrieved with default parameters", func() {
		t := s.T()

		resourceID := dbtest.CreateTestResource(t, s.DB, "Test Resource", 60)
		user1ID := dbtest.CreateTestUser(t, s.DB, "user1@example.com", string(user.RoleAdmin))
		user2ID := dbtest.CreateTestUser(t, s.DB, "user2@example.com", string(user.RoleAdmin))

		now := time.Now()
		reservation1ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user1ID,
			now.Add(-3*time.Hour), now.Add(-2*time.Hour), "confirmed")
		reservation2ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user2ID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token1 := authtest.LoginUser(t, s.Router, "user1@example.com", "password123")
		token2 := authtest.LoginUser(t, s.Router, "user2@example.com", "password123")

		// Create 2 reviews
		review1Req := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservation1ID).
			WithRating(5).
			WithComment("Excellent!").
			BuildCreateRequestDTO()

		review2Req := builder.NewReviewBuilder().
			WithResourceID(resourceID).
			WithReservationID(reservation2ID).
			WithRating(3).
			WithComment("Average").
			BuildCreateRequestDTO()

		httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, review1Req, token1)
		httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, review2Req, token2)

		// Get resource reviews
		url := fmt.Sprintf(resourceReviewsURL, resourceID.String())
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "Should return all reviews for resource")

		var actualRes struct {
			Reviews []*response.ReviewListItemResponse `json:"reviews"`
		}
		err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)
		require.Len(t, actualRes.Reviews, 2, "Should return 2 reviews")
	})

	s.Run("Normal case: Integration test (filter + pagination)", func() {
		t := s.T()

		type listTestCase struct {
			name          string
			queryParams   string
			expectedCount int
			validateFunc  func(t *testing.T, reviews []*response.ReviewListItemResponse)
		}

		testCases := []listTestCase{
			{
				name:          "Filter by minimum rating",
				queryParams:   "?min_rating=4",
				expectedCount: 2,
				validateFunc: func(t *testing.T, reviews []*response.ReviewListItemResponse) {
					for _, review := range reviews {
						require.GreaterOrEqual(t, review.Rating, int32(4))
					}
				},
			},
			{
				name:          "Filter by maximum rating",
				queryParams:   "?max_rating=3",
				expectedCount: 1,
				validateFunc: func(t *testing.T, reviews []*response.ReviewListItemResponse) {
					require.Equal(t, int32(2), reviews[0].Rating)
				},
			},
			{
				name:          "Limit results",
				queryParams:   "?limit=2",
				expectedCount: 2,
				validateFunc:  nil,
			},
		}

		for _, tc := range testCases {
			s.Run(tc.name, func() {
				// fresh seed per case (DB reset runs between subtests)
				resourceID := dbtest.CreateTestResource(t, s.DB, "Filter Test Resource", 60)
				user1ID := dbtest.CreateTestUser(t, s.DB, "filter1@example.com", string(user.RoleAdmin))
				user2ID := dbtest.CreateTestUser(t, s.DB, "filter2@example.com", string(user.RoleAdmin))
				user3ID := dbtest.CreateTestUser(t, s.DB, "filter3@example.com", string(user.RoleAdmin))

				now := time.Now()
				reservation1ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user1ID,
					now.Add(-4*time.Hour), now.Add(-3*time.Hour), "confirmed")
				reservation2ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user2ID,
					now.Add(-3*time.Hour), now.Add(-2*time.Hour), "confirmed")
				reservation3ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user3ID,
					now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

				token1 := authtest.LoginUser(t, s.Router, "filter1@example.com", "password123")
				token2 := authtest.LoginUser(t, s.Router, "filter2@example.com", "password123")
				token3 := authtest.LoginUser(t, s.Router, "filter3@example.com", "password123")

				for _, rv := range []struct {
					token   string
					resID   uuid.UUID
					rating  int
					comment string
				}{
					{token1, reservation1ID, 5, "Excellent!"},
					{token2, reservation2ID, 2, "Poor service"},
					{token3, reservation3ID, 4, "Good service"},
				} {
					req := builder.NewReviewBuilder().
						WithResourceID(resourceID).
						WithReservationID(rv.resID).
						WithRating(rv.rating).
						WithComment(rv.comment).
						BuildCreateRequestDTO()
					resp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, req, rv.token)
					require.Equal(t, http.StatusCreated, resp.Code, resp.Body.String())
				}

				url := fmt.Sprintf(resourceReviewsURL, resourceID.String()) + tc.queryParams
				w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
				require.Equal(t, http.StatusOK, w.Code)
				var actualRes struct {
					Reviews []*response.ReviewListItemResponse `json:"reviews"`
				}
				err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
				require.NoError(t, err)
				require.Len(t, actualRes.Reviews, tc.expectedCount)
				if tc.validateFunc != nil {
					tc.validateFunc(t, actualRes.Reviews)
				}
			})
		}
	})
}

// =============================================================================
// TestListUserReviews - User reviews list API tests
// =============================================================================

func (s *ReviewSuite) TestListUserReviews() {
	s.Run("Normal case: User reviews list retrieved successfully", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "listuser@example.com", string(user.RoleAdmin))
		resource1ID := dbtest.CreateTestResource(t, s.DB, "Test Resource 1", 60)
		resource2ID := dbtest.CreateTestResource(t, s.DB, "Test Resource 2", 60)

		now := time.Now()
		reservation1ID := dbtest.CreateTestReservation(t, s.DB, resource1ID, userID,
			now.Add(-4*time.Hour), now.Add(-3*time.Hour), "confirmed")
		reservation2ID := dbtest.CreateTestReservation(t, s.DB, resource2ID, userID,
			now.Add(-3*time.Hour), now.Add(-2*time.Hour), "confirmed")

		token := authtest.LoginUser(t, s.Router, "listuser@example.com", "password123")

		// Create 2 reviews by the same user
		review1Req := builder.NewReviewBuilder().
			WithResourceID(resource1ID).
			WithReservationID(reservation1ID).
			WithRating(5).
			WithComment("Great service!").
			BuildCreateRequestDTO()

		review2Req := builder.NewReviewBuilder().
			WithResourceID(resource2ID).
			WithReservationID(reservation2ID).
			WithRating(3).
			WithComment("Average service").
			BuildCreateRequestDTO()

		httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, review1Req, token)
		httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, review2Req, token)

		// Get user reviews
		url := fmt.Sprintf(userReviewsURL, userID.String())
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, token)
		require.Equal(t, http.StatusOK, w.Code, "Should return user reviews successfully")

		var actualRes struct {
			Reviews []*response.ReviewListItemResponse `json:"reviews"`
		}
		err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)
		require.Len(t, actualRes.Reviews, 2, "Should return 2 reviews for the user")
	})

	s.Run("Normal case: Integration test (pagination)", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "paginguser@example.com", string(user.RoleAdmin))
		token := authtest.LoginUser(t, s.Router, "paginguser@example.com", "password123")

		// Create multiple reviews for pagination test
		now := time.Now()
		for i := range 5 {
			resourceID := dbtest.CreateTestResource(t, s.DB, fmt.Sprintf("Resource %d", i), 60)
			reservationID := dbtest.CreateTestReservation(t, s.DB, resourceID, userID,
				now.Add(time.Duration(-6+i)*time.Hour), now.Add(time.Duration(-5+i)*time.Hour), "confirmed")

			reviewReq := builder.NewReviewBuilder().
				WithResourceID(resourceID).
				WithReservationID(reservationID).
				WithRating(4).
				WithComment(fmt.Sprintf("Review %d", i)).
				BuildCreateRequestDTO()

			resp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, reviewReq, token)
			require.Equal(t, http.StatusCreated, resp.Code)
		}

		// Test pagination with limit
		url := fmt.Sprintf(userReviewsURL, userID.String()) + "?limit=3"
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, token)
		require.Equal(t, http.StatusOK, w.Code)

		var actualRes struct {
			Reviews []*response.ReviewListItemResponse `json:"reviews"`
		}
		err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)
		require.Len(t, actualRes.Reviews, 3, "Should return limited number of reviews")
	})

	s.Run("Auth test - Unauthorized when not logged in", func() {
		t := s.T()

		userID := dbtest.CreateTestUser(t, s.DB, "authuser@example.com", string(user.RoleAdmin))
		url := fmt.Sprintf(userReviewsURL, userID.String())
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusUnauthorized, w.Code, "Should reject unauthorized access")
	})
}

// =============================================================================
// TestResourceRatingStats - Resource rating statistics API tests
// =============================================================================

func (s *ReviewSuite) TestResourceRatingStats() {
	s.Run("Normal case: Rating statistics retrieved successfully", func() {
		t := s.T()

		resourceID := dbtest.CreateTestResource(t, s.DB, "Stats Test Resource", 60)
		user1ID := dbtest.CreateTestUser(t, s.DB, "stats1@example.com", string(user.RoleAdmin))
		user2ID := dbtest.CreateTestUser(t, s.DB, "stats2@example.com", string(user.RoleAdmin))
		user3ID := dbtest.CreateTestUser(t, s.DB, "stats3@example.com", string(user.RoleAdmin))

		now := time.Now()
		reservation1ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user1ID,
			now.Add(-4*time.Hour), now.Add(-3*time.Hour), "confirmed")
		reservation2ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user2ID,
			now.Add(-3*time.Hour), now.Add(-2*time.Hour), "confirmed")
		reservation3ID := dbtest.CreateTestReservation(t, s.DB, resourceID, user3ID,
			now.Add(-2*time.Hour), now.Add(-1*time.Hour), "confirmed")

		token1 := authtest.LoginUser(t, s.Router, "stats1@example.com", "password123")
		token2 := authtest.LoginUser(t, s.Router, "stats2@example.com", "password123")
		token3 := authtest.LoginUser(t, s.Router, "stats3@example.com", "password123")

		// Create reviews with ratings: 5, 4, 3
		reviews := []struct {
			token         string
			reservationID uuid.UUID
			rating        int
			comment       string
		}{
			{token1, reservation1ID, 5, "Excellent!"},
			{token2, reservation2ID, 4, "Good service"},
			{token3, reservation3ID, 3, "Average"},
		}

		for _, review := range reviews {
			req := builder.NewReviewBuilder().
				WithResourceID(resourceID).
				WithReservationID(review.reservationID).
				WithRating(review.rating).
				WithComment(review.comment).
				BuildCreateRequestDTO()

			resp := httptest.PerformRequest(t, s.Router, http.MethodPost, reviewsURL, req, review.token)
			require.Equal(t, http.StatusCreated, resp.Code)
		}

		// Allow some time for stats to be updated
		time.Sleep(100 * time.Millisecond)

		// Get rating stats
		url := fmt.Sprintf(ratingStatsURL, resourceID.String())
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "Should retrieve rating stats successfully")

		var actualRes response.ResourceRatingStatsResponse
		err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)
		require.InDelta(t, 4.0, actualRes.AverageRating, 0.1) // (5+4+3)/3 = 4.0

		expected := &response.ResourceRatingStatsResponse{
			ResourceID:   resourceID.String(),
			TotalReviews: int32(3),
			Rating1Count: int32(0),
			Rating2Count: int32(0),
			Rating3Count: int32(1),
			Rating4Count: int32(1),
			Rating5Count: int32(1),
		}

		opts := []cmp.Option{
			cmpopts.IgnoreFields(response.ResourceRatingStatsResponse{}, "AverageRating", "UpdatedAt"),
		}

		if diff := cmp.Diff(expected, &actualRes, opts...); diff != "" {
			t.Errorf("Rating stats mismatch (-want +got):\n%s", diff)
		}
	})

	s.Run("Normal case: Returns empty stats for non-existent resource ID", func() {
		t := s.T()

		nonExistentResourceID := uuid.New().String()
		url := fmt.Sprintf(ratingStatsURL, nonExistentResourceID)
		w := httptest.PerformRequest(t, s.Router, http.MethodGet, url, nil, "")
		require.Equal(t, http.StatusOK, w.Code, "Should return empty stats for non-existent resource")

		var actualRes response.ResourceRatingStatsResponse
		err := httptest.DecodeResponseBody(t, w.Body, &actualRes)
		require.NoError(t, err)

		expected := &response.ResourceRatingStatsResponse{
			ResourceID:    nonExistentResourceID,
			TotalReviews:  int32(0),
			AverageRating: 0.0,
		}

		opts := []cmp.Option{
			cmpopts.IgnoreFields(response.ResourceRatingStatsResponse{}, "Rating1Count", "Rating2Count", "Rating3Count", "Rating4Count", "Rating5Count", "UpdatedAt"),
		}

		if diff := cmp.Diff(expected, &actualRes, opts...); diff != "" {
			t.Errorf("Empty stats mismatch (-want +got):\n%s", diff)
		}
	})
}
