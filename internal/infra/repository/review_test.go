//go:build unit

package repository_test

import (
	"context"
	"errors"
	"testing"

	"gin-clean-starter/internal/domain/review"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/repository"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/tests/common/builder"
	repositorymock "gin-clean-starter/tests/mock/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// =============================================================================
// Create Review Tests
// =============================================================================

func TestRepository_Create(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockReviewWriteQueries, *review.Review, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: review created successfully",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				mock.EXPECT().CreateReview(ctx, tx, gomock.Any()).Return(rev.ID(), nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				mock.EXPECT().CreateReview(ctx, tx, gomock.Any()).Return(uuid.Nil, errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
		{
			name: "error: duplicate review error",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				dup := &pgconn.PgError{Code: "23505", Message: "duplicate key value violates unique constraint"}
				mock.EXPECT().CreateReview(ctx, tx, gomock.Any()).Return(uuid.Nil, dup)
			},
			expectedError: true,
			expectKind:    infra.KindDuplicateKey,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockReviewWriteQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewReviewRepository(mockQueries, mockDB)

			domainReview, err := builder.NewReviewBuilder().BuildDomain()
			require.NoError(t, err)

			tc.setupMock(mockQueries, domainReview, mockDB)

			reviewID, actualError := repo.Create(ctx, mockDB, domainReview)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
				assert.Equal(t, uuid.Nil, reviewID, "reviewID should be nil when error occurs")
			} else {
				assert.NoError(t, actualError)
				assert.NotEqual(t, uuid.Nil, reviewID)
			}
		})
	}
}

// =============================================================================
// Update Review Tests
// =============================================================================

func TestRepository_Update(t *testing.T) {
	ctx := context.Background()

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockReviewWriteQueries, *review.Review, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: review updated successfully",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				mock.EXPECT().UpdateReview(ctx, tx, gomock.Any()).Return(int32(1), nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				mock.EXPECT().UpdateReview(ctx, tx, gomock.Any()).Return(int32(0), errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
		{
			name: "error: review not found",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, rev *review.Review, tx sqlc.DBTX) {
				mock.EXPECT().UpdateReview(ctx, tx, gomock.Any()).Return(int32(0), nil)
			},
			expectedError: true,
			expectKind:    infra.KindNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockReviewWriteQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewReviewRepository(mockQueries, mockDB)

			domainReview, err := builder.NewReviewBuilder().BuildDomain()
			require.NoError(t, err)

			tc.setupMock(mockQueries, domainReview, mockDB)

			actualError := repo.Update(ctx, mockDB, domainReview.ID(), domainReview)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
			} else {
				assert.NoError(t, actualError)
			}
		})
	}
}

// =============================================================================
// Delete Review Tests
// =============================================================================

func TestRepository_Delete(t *testing.T) {
	ctx := context.Background()
	reviewID := uuid.New()

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockReviewWriteQueries, uuid.UUID, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: review deleted successfully",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, id uuid.UUID, tx sqlc.DBTX) {
				mock.EXPECT().DeleteReview(ctx, tx, id).Return(int32(1), nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, id uuid.UUID, tx sqlc.DBTX) {
				mock.EXPECT().DeleteReview(ctx, tx, id).Return(int32(0), errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
		{
			name: "error: review not found",
			setupMock: func(mock *repositorymock.MockReviewWriteQueries, id uuid.UUID, tx sqlc.DBTX) {
				mock.EXPECT().DeleteReview(ctx, tx, id).Return(int32(0), nil)
			},
			expectedError: true,
			expectKind:    infra.KindNotFound,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockReviewWriteQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewReviewRepository(mockQueries, mockDB)

			tc.setupMock(mockQueries, reviewID, mockDB)

			actualError := repo.Delete(ctx, mockDB, reviewID)

			if tc.expectedError {
				require.Error(t, actualError)
				if tc.expectKind != "" {
					assert.True(t, infra.IsKind(actualError, tc.expectKind), "expected kind [%v] but got [%T] (%v)", tc.expectKind, actualError, actualError)
				}
			} else {
				assert.NoError(t, actualError)
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
	panic("mockDBTX.QueryRow was called unexpectedly. Use sqlc mock instead.")
}
