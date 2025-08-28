//go:build unit

package readstore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/readstore"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	readstoremock "gin-clean-starter/tests/mock/readstore"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	errDBConnectionLost = errors.New("database connection lost")
)

// =============================================================================
// FindByID Tests
// =============================================================================

func TestReadStore_FindByID(t *testing.T) {
	ctx := context.Background()
	reviewID := uuid.New()

	testCases := []struct {
		name          string
		setupMock     func(*readstoremock.MockReviewReadQueries, uuid.UUID)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: review found",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, id uuid.UUID) {
				expectedRow := sqlc.GetReviewViewByIDRow{
					ID:            id,
					UserID:        uuid.New(),
					UserEmail:     "test@example.com",
					ResourceID:    uuid.New(),
					ResourceName:  "Test Resource",
					ReservationID: uuid.New(),
					Rating:        5,
					Comment:       "Great service!",
					CreatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
					UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				mock.EXPECT().GetReviewViewByID(ctx, gomock.Any(), id).Return(expectedRow, nil)
			},
			expectedError: false,
		},
		{
			name: "error: review not found",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, id uuid.UUID) {
				mock.EXPECT().GetReviewViewByID(ctx, gomock.Any(), id).Return(sqlc.GetReviewViewByIDRow{}, pgx.ErrNoRows)
			},
			expectedError: true,
			expectKind:    infra.KindNotFound,
		},
		{
			name: "error: database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, id uuid.UUID) {
				mock.EXPECT().GetReviewViewByID(ctx, gomock.Any(), id).Return(sqlc.GetReviewViewByIDRow{}, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)

			tc.setupMock(mockQueries, reviewID)

			result, actualError := store.FindByID(ctx, reviewID)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Nil(t, result, "result should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				require.NotNil(t, result)
				assert.Equal(t, reviewID, result.ID)
			}
		})
	}
}

// =============================================================================
// FindByResourceFirstPage Tests
// =============================================================================

func TestReadStore_FindByResourceFirstPage(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()
	limit := int32(20)

	testCases := []struct {
		name          string
		minRating     *int
		maxRating     *int
		setupMock     func(*readstoremock.MockReviewReadQueries, uuid.UUID, int32, *int, *int)
		expectedCount int
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: reviews found without rating filters",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, lmt int32, minRat, maxRat *int) {
				expectedRows := []sqlc.GetReviewsByResourceFirstPageRow{
					{
						ID:        uuid.New(),
						UserEmail: "user1@example.com",
						Rating:    5,
						Comment:   "Excellent!",
						CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
					},
					{
						ID:        uuid.New(),
						UserEmail: "user2@example.com",
						Rating:    4,
						Comment:   "Good service",
						CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
					},
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(expectedRows, nil)
			},
			expectedCount: 2,
			expectedError: false,
		},
		{
			name:      "success: reviews found with rating filters",
			minRating: func() *int { i := 4; return &i }(),
			maxRating: func() *int { i := 5; return &i }(),
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, lmt int32, minRat, maxRat *int) {
				expectedRows := []sqlc.GetReviewsByResourceFirstPageRow{
					{
						ID:        uuid.New(),
						UserEmail: "user1@example.com",
						Rating:    5,
						Comment:   "Excellent!",
						CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
					},
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(expectedRows, nil)
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "success: no reviews found",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, lmt int32, minRat, maxRat *int) {
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByResourceFirstPageRow{}, nil)
			},
			expectedCount: 0,
			expectedError: false,
		},
		{
			name: "error: database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, lmt int32, minRat, maxRat *int) {
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID, limit, tc.minRating, tc.maxRating)

			results, actualError := store.FindByResourceFirstPage(ctx, resourceID, limit, tc.minRating, tc.maxRating)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Nil(t, results, "results should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				require.NotNil(t, results)
				assert.Len(t, results, tc.expectedCount)
			}
		})
	}
}

// =============================================================================
// FindByResourceKeyset Tests
// =============================================================================

func TestReadStore_FindByResourceKeyset(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()
	lastCreatedAt := time.Now()
	lastID := uuid.New()
	limit := int32(20)

	testCases := []struct {
		name          string
		setupMock     func(*readstoremock.MockReviewReadQueries, uuid.UUID, time.Time, uuid.UUID, int32)
		expectedCount int
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: reviews found with keyset pagination",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, createdAt time.Time, id uuid.UUID, lmt int32) {
				expectedRows := []sqlc.GetReviewsByResourceKeysetRow{
					{
						ID:        uuid.New(),
						UserEmail: "user1@example.com",
						Rating:    5,
						Comment:   "Excellent!",
						CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
					},
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(expectedRows, nil)
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "error: database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID, createdAt time.Time, id uuid.UUID, lmt int32) {
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID, lastCreatedAt, lastID, limit)

			results, actualError := store.FindByResourceKeyset(ctx, resourceID, lastCreatedAt, lastID, limit, nil, nil)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Nil(t, results, "results should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				require.NotNil(t, results)
				assert.Len(t, results, tc.expectedCount)
			}
		})
	}
}

// =============================================================================
// FindByUserFirstPage Tests
// =============================================================================

func TestReadStore_FindByUserFirstPage(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	limit := int32(20)

	testCases := []struct {
		name          string
		setupMock     func(*readstoremock.MockReviewReadQueries, uuid.UUID, int32)
		expectedCount int
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: user reviews found",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, usrID uuid.UUID, lmt int32) {
				expectedRows := []sqlc.GetReviewsByUserFirstPageRow{
					{
						ID:        uuid.New(),
						UserEmail: "user@example.com",
						Rating:    4,
						Comment:   "Good service",
						CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
					},
				}
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(expectedRows, nil)
			},
			expectedCount: 1,
			expectedError: false,
		},
		{
			name: "error: database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, usrID uuid.UUID, lmt int32) {
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)

			tc.setupMock(mockQueries, userID, limit)

			results, actualError := store.FindByUserFirstPage(ctx, userID, limit)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Nil(t, results, "results should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				require.NotNil(t, results)
				assert.Len(t, results, tc.expectedCount)
			}
		})
	}
}

// =============================================================================
// GetResourceRatingStats Tests
// =============================================================================

func TestReadStore_GetResourceRatingStats(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()

	testCases := []struct {
		name          string
		setupMock     func(*readstoremock.MockReviewReadQueries, uuid.UUID)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: rating stats found",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID) {
				avgRating := pgtype.Numeric{Valid: true}
				avgRating.Scan(4.5)
				expectedRow := sqlc.ResourceRatingStats{
					ResourceID:    resID,
					TotalReviews:  10,
					AverageRating: avgRating,
					Rating1Count:  1,
					Rating2Count:  1,
					Rating3Count:  2,
					Rating4Count:  3,
					Rating5Count:  3,
					UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resID).Return(expectedRow, nil)
			},
			expectedError: false,
		},
		{
			name: "success: no stats found returns zero stats",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID) {
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resID).Return(sqlc.ResourceRatingStats{}, pgx.ErrNoRows)
			},
			expectedError: false,
		},
		{
			name: "error: database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resID uuid.UUID) {
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resID).Return(sqlc.ResourceRatingStats{}, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID)

			result, actualError := store.GetResourceRatingStats(ctx, resourceID)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Nil(t, result, "result should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				require.NotNil(t, result)
				assert.Equal(t, resourceID, result.ResourceID)
			}
		})
	}
}

// =============================================================================
// Test Helper Functions
// =============================================================================

// mockDBTX is a mock implementation of sqlc.DBTX interface
type mockDBTX struct{}

func (m *mockDBTX) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (m *mockDBTX) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (m *mockDBTX) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
