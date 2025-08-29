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

	t.Run("Filter Conditions", func(t *testing.T) {
		testFilterCases(t, ctx)
	})

	t.Run("Pagination", func(t *testing.T) {
		testPaginationCases(t, ctx)
	})

	t.Run("Complex Conditions", func(t *testing.T) {
		testComplexConditionCases(t, ctx)
	})

	t.Run("Error Cases", func(t *testing.T) {
		testErrorCases(t, ctx)
	})
}

// =============================================================================
// Test Helper Functions for Structured Testing
// =============================================================================

type ReviewTestCase struct {
	name          string
	minRating     *int
	maxRating     *int
	limit         int32
	setupMock     func(mock *readstoremock.MockReviewReadQueries)
	expectedCount int
	expectedError bool
	expectKind    infra.RepositoryErrorKind
}

func testFilterCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewTestCase{
		{
			name:  "no filters - all results",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{
					createReviewRow(5, "Great!"),
					createReviewRow(3, "OK"),
					createReviewRow(1, "Bad"),
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
		{
			name:      "rating filter - minRating=4",
			minRating: intPtr(4),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{
					createReviewRow(5, "Great!"),
					createReviewRow(4, "Good"),
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 2,
		},
		{
			name:      "rating filter - maxRating=3",
			maxRating: intPtr(3),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{
					createReviewRow(3, "OK"),
					createReviewRow(1, "Bad"),
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 2,
		},
		{
			name:      "rating range filter - minRating=3, maxRating=4",
			minRating: intPtr(3),
			maxRating: intPtr(4),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{
					createReviewRow(4, "Good"),
					createReviewRow(3, "OK"),
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 2,
		},
		{
			name:      "no results - minRating too high",
			minRating: intPtr(6),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByResourceFirstPageRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runReviewTestCases(t, ctx, testCases)
}

func testPaginationCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewTestCase{
		{
			name:  "pagination - limit=1",
			limit: 1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{createReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:  "pagination - limit=100 (large)",
			limit: 100,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := make([]sqlc.GetReviewsByResourceFirstPageRow, 10)
				for i := 0; i < 10; i++ {
					rows[i] = createReviewRow(5-i%5, "Review")
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 10,
		},
		{
			name:  "empty results",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByResourceFirstPageRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runReviewTestCases(t, ctx, testCases)
}

func testComplexConditionCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewTestCase{
		{
			name:      "combined - filter + small limit",
			minRating: intPtr(4),
			limit:     1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{createReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:      "combined - range filter + pagination",
			minRating: intPtr(2),
			maxRating: intPtr(4),
			limit:     5,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceFirstPageRow{
					createReviewRow(4, "Good"),
					createReviewRow(3, "OK"),
					createReviewRow(2, "Fair"),
				}
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
	}

	runReviewTestCases(t, ctx, testCases)
}

func testErrorCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewTestCase{
		{
			name:  "database error",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceFirstPage(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	runReviewTestCases(t, ctx, testCases)
}

func runReviewTestCases(t *testing.T, ctx context.Context, testCases []ReviewTestCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)
			resourceID := uuid.New()

			tc.setupMock(mockQueries)

			results, actualError := store.FindByResourceFirstPage(ctx, resourceID, tc.limit, tc.minRating, tc.maxRating)

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

func createReviewRow(rating int, comment string) sqlc.GetReviewsByResourceFirstPageRow {
	return sqlc.GetReviewsByResourceFirstPageRow{
		ID:        uuid.New(),
		UserEmail: "user@example.com",
		Rating:    int32(rating),
		Comment:   comment,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

func intPtr(i int) *int {
	return &i
}

// =============================================================================
// FindByResourceKeyset Tests
// =============================================================================

func TestReadStore_FindByResourceKeyset(t *testing.T) {
	ctx := context.Background()

	t.Run("Filter Conditions", func(t *testing.T) {
		testKeysetFilterCases(t, ctx)
	})

	t.Run("Pagination", func(t *testing.T) {
		testKeysetPaginationCases(t, ctx)
	})

	t.Run("Complex Conditions", func(t *testing.T) {
		testKeysetComplexConditionCases(t, ctx)
	})

	t.Run("Error Cases", func(t *testing.T) {
		testKeysetErrorCases(t, ctx)
	})
}

func testKeysetFilterCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewKeysetTestCase{
		{
			name:  "no filters - all results",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{
					createKeysetReviewRow(5, "Great!"),
					createKeysetReviewRow(3, "OK"),
					createKeysetReviewRow(1, "Bad"),
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
		{
			name:      "rating filter - minRating=4",
			minRating: intPtr(4),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{
					createKeysetReviewRow(5, "Great!"),
					createKeysetReviewRow(4, "Good"),
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 2,
		},
		{
			name:      "rating range filter - minRating=2, maxRating=4",
			minRating: intPtr(2),
			maxRating: intPtr(4),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{
					createKeysetReviewRow(4, "Good"),
					createKeysetReviewRow(3, "OK"),
					createKeysetReviewRow(2, "Fair"),
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
		{
			name:      "no results - minRating too high",
			minRating: intPtr(6),
			limit:     20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByResourceKeysetRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runKeysetReviewTestCases(t, ctx, testCases)
}

func testKeysetPaginationCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewKeysetTestCase{
		{
			name:  "pagination - limit=1",
			limit: 1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{createKeysetReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:  "pagination - limit=100 (large)",
			limit: 100,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := make([]sqlc.GetReviewsByResourceKeysetRow, 10)
				for i := 0; i < 10; i++ {
					rows[i] = createKeysetReviewRow(5-i%5, "Review")
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 10,
		},
		{
			name:  "empty results",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByResourceKeysetRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runKeysetReviewTestCases(t, ctx, testCases)
}

func testKeysetComplexConditionCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewKeysetTestCase{
		{
			name:      "combined - filter + small limit",
			minRating: intPtr(4),
			limit:     1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{createKeysetReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:      "combined - range filter + pagination",
			minRating: intPtr(2),
			maxRating: intPtr(4),
			limit:     5,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByResourceKeysetRow{
					createKeysetReviewRow(4, "Good"),
					createKeysetReviewRow(3, "OK"),
					createKeysetReviewRow(2, "Fair"),
				}
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
	}

	runKeysetReviewTestCases(t, ctx, testCases)
}

func testKeysetErrorCases(t *testing.T, ctx context.Context) {
	testCases := []ReviewKeysetTestCase{
		{
			name:  "database error",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByResourceKeyset(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	runKeysetReviewTestCases(t, ctx, testCases)
}

type ReviewKeysetTestCase struct {
	name          string
	minRating     *int
	maxRating     *int
	limit         int32
	setupMock     func(mock *readstoremock.MockReviewReadQueries)
	expectedCount int
	expectedError bool
	expectKind    infra.RepositoryErrorKind
}

func runKeysetReviewTestCases(t *testing.T, ctx context.Context, testCases []ReviewKeysetTestCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)
			resourceID := uuid.New()
			lastCreatedAt := time.Now()
			lastID := uuid.New()

			tc.setupMock(mockQueries)

			results, actualError := store.FindByResourceKeyset(ctx, resourceID, lastCreatedAt, lastID, tc.limit, tc.minRating, tc.maxRating)

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

func createKeysetReviewRow(rating int, comment string) sqlc.GetReviewsByResourceKeysetRow {
	return sqlc.GetReviewsByResourceKeysetRow{
		ID:        uuid.New(),
		UserEmail: "user@example.com",
		Rating:    int32(rating),
		Comment:   comment,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// =============================================================================
// FindByUserFirstPage Tests
// =============================================================================

func TestReadStore_FindByUserFirstPage(t *testing.T) {
	ctx := context.Background()

	t.Run("Pagination", func(t *testing.T) {
		testUserFirstPagePaginationCases(t, ctx)
	})

	t.Run("Error Cases", func(t *testing.T) {
		testUserFirstPageErrorCases(t, ctx)
	})
}

func testUserFirstPagePaginationCases(t *testing.T, ctx context.Context) {
	testCases := []UserReviewTestCase{
		{
			name:  "pagination - standard limit",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByUserFirstPageRow{
					createUserReviewRow(5, "Great service!"),
					createUserReviewRow(4, "Good experience"),
					createUserReviewRow(3, "Average"),
				}
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 3,
		},
		{
			name:  "pagination - limit=1",
			limit: 1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByUserFirstPageRow{createUserReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:  "pagination - limit=100 (large)",
			limit: 100,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := make([]sqlc.GetReviewsByUserFirstPageRow, 15)
				for i := 0; i < 15; i++ {
					rows[i] = createUserReviewRow(5-i%5, "Review")
				}
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 15,
		},
		{
			name:  "empty results - user has no reviews",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByUserFirstPageRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runUserReviewTestCases(t, ctx, testCases)
}

func testUserFirstPageErrorCases(t *testing.T, ctx context.Context) {
	testCases := []UserReviewTestCase{
		{
			name:  "database error",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByUserFirstPage(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	runUserReviewTestCases(t, ctx, testCases)
}

type UserReviewTestCase struct {
	name          string
	limit         int32
	setupMock     func(mock *readstoremock.MockReviewReadQueries)
	expectedCount int
	expectedError bool
	expectKind    infra.RepositoryErrorKind
}

func runUserReviewTestCases(t *testing.T, ctx context.Context, testCases []UserReviewTestCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)
			userID := uuid.New()

			tc.setupMock(mockQueries)

			results, actualError := store.FindByUserFirstPage(ctx, userID, tc.limit)

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

func createUserReviewRow(rating int, comment string) sqlc.GetReviewsByUserFirstPageRow {
	return sqlc.GetReviewsByUserFirstPageRow{
		ID:        uuid.New(),
		UserEmail: "user@example.com",
		Rating:    int32(rating),
		Comment:   comment,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// =============================================================================
// FindByUserKeyset Tests
// =============================================================================

func TestReadStore_FindByUserKeyset(t *testing.T) {
	ctx := context.Background()

	t.Run("Pagination", func(t *testing.T) {
		testUserKeysetPaginationCases(t, ctx)
	})

	t.Run("Error Cases", func(t *testing.T) {
		testUserKeysetErrorCases(t, ctx)
	})
}

func testUserKeysetPaginationCases(t *testing.T, ctx context.Context) {
	testCases := []UserKeysetTestCase{
		{
			name:  "pagination - standard keyset",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByUserKeysetRow{
					createUserKeysetReviewRow(4, "Good service"),
					createUserKeysetReviewRow(3, "OK service"),
				}
				mock.EXPECT().GetReviewsByUserKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 2,
		},
		{
			name:  "pagination - limit=1",
			limit: 1,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				rows := []sqlc.GetReviewsByUserKeysetRow{createUserKeysetReviewRow(5, "Great!")}
				mock.EXPECT().GetReviewsByUserKeyset(ctx, gomock.Any(), gomock.Any()).Return(rows, nil)
			},
			expectedCount: 1,
		},
		{
			name:  "empty results - no more reviews",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByUserKeyset(ctx, gomock.Any(), gomock.Any()).Return([]sqlc.GetReviewsByUserKeysetRow{}, nil)
			},
			expectedCount: 0,
		},
	}

	runUserKeysetTestCases(t, ctx, testCases)
}

func testUserKeysetErrorCases(t *testing.T, ctx context.Context) {
	testCases := []UserKeysetTestCase{
		{
			name:  "database error",
			limit: 20,
			setupMock: func(mock *readstoremock.MockReviewReadQueries) {
				mock.EXPECT().GetReviewsByUserKeyset(ctx, gomock.Any(), gomock.Any()).Return(nil, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	runUserKeysetTestCases(t, ctx, testCases)
}

type UserKeysetTestCase struct {
	name          string
	limit         int32
	setupMock     func(mock *readstoremock.MockReviewReadQueries)
	expectedCount int
	expectedError bool
	expectKind    infra.RepositoryErrorKind
}

func runUserKeysetTestCases(t *testing.T, ctx context.Context, testCases []UserKeysetTestCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)
			userID := uuid.New()
			lastCreatedAt := time.Now()
			lastID := uuid.New()

			tc.setupMock(mockQueries)

			results, actualError := store.FindByUserKeyset(ctx, userID, lastCreatedAt, lastID, tc.limit)

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

func createUserKeysetReviewRow(rating int, comment string) sqlc.GetReviewsByUserKeysetRow {
	return sqlc.GetReviewsByUserKeysetRow{
		ID:        uuid.New(),
		UserEmail: "user@example.com",
		Rating:    int32(rating),
		Comment:   comment,
		CreatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
}

// =============================================================================
// GetResourceRatingStats Tests
// =============================================================================

func TestReadStore_GetResourceRatingStats(t *testing.T) {
	ctx := context.Background()

	t.Run("Normal Cases", func(t *testing.T) {
		testRatingStatsNormalCases(t, ctx)
	})

	t.Run("Error Cases", func(t *testing.T) {
		testRatingStatsErrorCases(t, ctx)
	})
}

func testRatingStatsNormalCases(t *testing.T, ctx context.Context) {
	testCases := []RatingStatsTestCase{
		{
			name: "success - stats found with reviews",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID) {
				var avgRating pgtype.Numeric
				err := avgRating.Scan("4.5")
				if err != nil {
					t.Fatalf("Failed to scan numeric: %v", err)
				}
				expectedRow := sqlc.ResourceRatingStats{
					ResourceID:    resourceID,
					TotalReviews:  10,
					AverageRating: avgRating,
					Rating1Count:  1,
					Rating2Count:  1,
					Rating3Count:  2,
					Rating4Count:  3,
					Rating5Count:  3,
					UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resourceID).Return(expectedRow, nil)
			},
			expectedTotalReviews: 10,
			expectedAvgRating:    4.5,
		},
		{
			name: "success - no stats found returns zero stats",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID) {
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resourceID).Return(sqlc.ResourceRatingStats{}, pgx.ErrNoRows)
			},
			expectedTotalReviews: 0,
			expectedAvgRating:    0.0,
		},
		{
			name: "success - stats with only 5-star reviews",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID) {
				var avgRating pgtype.Numeric
				err := avgRating.Scan("5.0")
				if err != nil {
					t.Fatalf("Failed to scan numeric: %v", err)
				}
				expectedRow := sqlc.ResourceRatingStats{
					ResourceID:    resourceID,
					TotalReviews:  5,
					AverageRating: avgRating,
					Rating1Count:  0,
					Rating2Count:  0,
					Rating3Count:  0,
					Rating4Count:  0,
					Rating5Count:  5,
					UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resourceID).Return(expectedRow, nil)
			},
			expectedTotalReviews: 5,
			expectedAvgRating:    5.0,
		},
		{
			name: "success - stats with only 1-star reviews",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID) {
				var avgRating pgtype.Numeric
				err := avgRating.Scan("1.0")
				if err != nil {
					t.Fatalf("Failed to scan numeric: %v", err)
				}
				expectedRow := sqlc.ResourceRatingStats{
					ResourceID:    resourceID,
					TotalReviews:  3,
					AverageRating: avgRating,
					Rating1Count:  3,
					Rating2Count:  0,
					Rating3Count:  0,
					Rating4Count:  0,
					Rating5Count:  0,
					UpdatedAt:     pgtype.Timestamptz{Time: time.Now(), Valid: true},
				}
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resourceID).Return(expectedRow, nil)
			},
			expectedTotalReviews: 3,
			expectedAvgRating:    1.0,
		},
	}

	runRatingStatsTestCases(t, ctx, testCases)
}

func testRatingStatsErrorCases(t *testing.T, ctx context.Context) {
	testCases := []RatingStatsTestCase{
		{
			name: "database error",
			setupMock: func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID) {
				mock.EXPECT().GetResourceRatingStats(ctx, gomock.Any(), resourceID).Return(sqlc.ResourceRatingStats{}, errDBConnectionLost)
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	runRatingStatsTestCases(t, ctx, testCases)
}

type RatingStatsTestCase struct {
	name                 string
	setupMock            func(mock *readstoremock.MockReviewReadQueries, resourceID uuid.UUID)
	expectedTotalReviews int32
	expectedAvgRating    float64
	expectedError        bool
	expectKind           infra.RepositoryErrorKind
}

func runRatingStatsTestCases(t *testing.T, ctx context.Context, testCases []RatingStatsTestCase) {
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := readstoremock.NewMockReviewReadQueries(ctrl)
			mockDB := &mockDBTX{}
			store := readstore.NewReviewReadStore(mockQueries, mockDB)
			resourceID := uuid.New()

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
				assert.Equal(t, tc.expectedTotalReviews, result.TotalReviews)
				assert.InDelta(t, tc.expectedAvgRating, result.AverageRating, 0.001)
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
