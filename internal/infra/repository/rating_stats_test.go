//go:build unit

package repository_test

import (
	"context"
	"errors"
	"testing"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/repository"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	repositorymock "gin-clean-starter/tests/mock/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// =============================================================================
// ApplyOnCreate Tests
// =============================================================================

func TestRepository_ApplyOnCreate(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()
	rating := 5

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockRatingStatsQueries, uuid.UUID, int, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: rating stats applied on create successfully",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, rating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnCreate(ctx, tx, gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, rating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnCreate(ctx, tx, gomock.Any()).Return(errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockRatingStatsQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewRatingStatsRepository(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID, rating, mockDB)

			actualError := repo.ApplyOnCreate(ctx, mockDB, resourceID, rating)

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
// ApplyOnUpdate Tests
// =============================================================================

func TestRepository_ApplyOnUpdate(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()
	oldRating := 3
	newRating := 5

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockRatingStatsQueries, uuid.UUID, int, int, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: rating stats applied on update successfully",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, oldRating, newRating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnUpdate(ctx, tx, gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, oldRating, newRating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnUpdate(ctx, tx, gomock.Any()).Return(errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockRatingStatsQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewRatingStatsRepository(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID, oldRating, newRating, mockDB)

			actualError := repo.ApplyOnUpdate(ctx, mockDB, resourceID, oldRating, newRating)

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
// ApplyOnDelete Tests
// =============================================================================

func TestRepository_ApplyOnDelete(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()
	oldRating := 4

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockRatingStatsQueries, uuid.UUID, int, sqlc.DBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: rating stats applied on delete successfully",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, oldRating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnDelete(ctx, tx, gomock.Any()).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, oldRating int, tx sqlc.DBTX) {
				mock.EXPECT().ApplyResourceRatingStatsOnDelete(ctx, tx, gomock.Any()).Return(errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mockQueries := repositorymock.NewMockRatingStatsQueries(ctrl)
			mockDB := &mockDBTX{}
			repo := repository.NewRatingStatsRepository(mockQueries, mockDB)

			tc.setupMock(mockQueries, resourceID, oldRating, mockDB)

			actualError := repo.ApplyOnDelete(ctx, mockDB, resourceID, oldRating)

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
