//go:build unit

package repository_test

import (
	"context"
	"errors"
	"testing"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/repository"
	repositorymock "gin-clean-starter/tests/mock/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// =============================================================================
// RecalcResourceRatingStats Tests
// =============================================================================

func TestRatingStatsRepository_RecalcResourceRatingStats(t *testing.T) {
	ctx := context.Background()
	resourceID := uuid.New()

	testCases := []struct {
		name          string
		setupMock     func(*repositorymock.MockRatingStatsQueries, uuid.UUID, *mockDBTX)
		expectedError bool
		expectKind    infra.RepositoryErrorKind
	}{
		{
			name: "success: rating stats recalculated successfully",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, tx *mockDBTX) {
				mock.EXPECT().RecalcResourceRatingStats(ctx, tx, resID).Return(nil)
			},
			expectedError: false,
		},
		{
			name: "error: database error occurs",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, tx *mockDBTX) {
				mock.EXPECT().RecalcResourceRatingStats(ctx, tx, resID).Return(errors.New("database connection error"))
			},
			expectedError: true,
			expectKind:    infra.KindDBFailure,
		},
		{
			name: "error: resource not found",
			setupMock: func(mock *repositorymock.MockRatingStatsQueries, resID uuid.UUID, tx *mockDBTX) {
				mock.EXPECT().RecalcResourceRatingStats(ctx, tx, resID).Return(errors.New("resource not found"))
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

			tc.setupMock(mockQueries, resourceID, mockDB)

			actualError := repo.RecalcResourceRatingStats(ctx, mockDB, resourceID)

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
