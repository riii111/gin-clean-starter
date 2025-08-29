//go:build unit

package repository

import (
	"context"
	"testing"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockUserWriteQueries struct {
	mock.Mock
}

func (m *MockUserWriteQueries) UpdateUserLastLogin(ctx context.Context, db sqlc.DBTX, id uuid.UUID) error {
	args := m.Called(ctx, db, id)
	return args.Error(0)
}

func (m *MockUserWriteQueries) CreateUser(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateUserParams) (uuid.UUID, error) {
	args := m.Called(ctx, db, arg)
	return args.Get(0).(uuid.UUID), args.Error(1)
}

// sqlc.DBTX implementation for MockUserWriteQueries
func (m *MockUserWriteQueries) Exec(ctx context.Context, query string, args ...interface{}) (pgconn.CommandTag, error) {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0).(pgconn.CommandTag), mockArgs.Error(1)
}

func (m *MockUserWriteQueries) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0).(pgx.Rows), mockArgs.Error(1)
}

func (m *MockUserWriteQueries) QueryRow(ctx context.Context, query string, args ...interface{}) pgx.Row {
	mockArgs := m.Called(ctx, query, args)
	return mockArgs.Get(0).(pgx.Row)
}

func TestUpdateLastLogin(t *testing.T) {
	testUserID := uuid.New()

	tests := []struct {
		name      string
		userID    uuid.UUID
		mockError error
		wantError bool
	}{
		{
			name:      "success",
			userID:    testUserID,
			mockError: nil,
			wantError: false,
		},
		{
			name:      "database error",
			userID:    testUserID,
			mockError: assert.AnError,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueries := new(MockUserWriteQueries)
			mockQueries.On("UpdateUserLastLogin", mock.Anything, mock.Anything, tt.userID).Return(tt.mockError)

			repo := NewUserRepository(mockQueries)

			err := repo.UpdateLastLogin(context.Background(), mockQueries, tt.userID)

			if tt.wantError {
				assert.Error(t, err)
				assert.True(t, infra.IsKind(err, infra.KindDBFailure))
			} else {
				assert.NoError(t, err)
			}

			mockQueries.AssertExpectations(t)
		})
	}
}
