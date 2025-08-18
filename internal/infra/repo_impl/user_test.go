//go:build unit

package repo_impl

import (
	"context"
	"database/sql"
	"testing"

	"gin-clean-starter/internal/domain/user"
	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/tests/common/builder"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockQueries struct {
	mock.Mock
}

func (m *MockQueries) FindUserByEmail(ctx context.Context, db sqlc.DBTX, email string) (sqlc.Users, error) {
	args := m.Called(ctx, db, email)
	return args.Get(0).(sqlc.Users), args.Error(1)
}

func (m *MockQueries) FindUserByID(ctx context.Context, db sqlc.DBTX, id uuid.UUID) (sqlc.FindUserByIDRow, error) {
	args := m.Called(ctx, db, id)
	return args.Get(0).(sqlc.FindUserByIDRow), args.Error(1)
}

func (m *MockQueries) UpdateUserLastLogin(ctx context.Context, db sqlc.DBTX, id uuid.UUID) error {
	args := m.Called(ctx, db, id)
	return args.Error(0)
}

func TestFindByEmail(t *testing.T) {
	testUser := builder.NewUserBuilder().BuildInfra()
	inactiveUser := builder.NewUserBuilder().AsInactive().BuildInfra()

	tests := []struct {
		name       string
		email      string
		mockReturn sqlc.Users
		mockError  error
		wantUser   bool
		wantHash   string
		wantError  bool
	}{
		{
			name:       "success - active user",
			email:      testUser.Email,
			mockReturn: testUser,
			mockError:  nil,
			wantUser:   true,
			wantHash:   testUser.PasswordHash,
			wantError:  false,
		},
		{
			name:       "success - inactive user (for validation)",
			email:      inactiveUser.Email,
			mockReturn: inactiveUser,
			mockError:  nil,
			wantUser:   true,
			wantHash:   inactiveUser.PasswordHash,
			wantError:  false,
		},
		{
			name:       "user not found",
			email:      "notfound@example.com",
			mockReturn: sqlc.Users{},
			mockError:  sql.ErrNoRows,
			wantUser:   false,
			wantHash:   "",
			wantError:  true,
		},
		{
			name:       "database error",
			email:      testUser.Email,
			mockReturn: sqlc.Users{},
			mockError:  assert.AnError,
			wantUser:   false,
			wantHash:   "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueries := new(MockQueries)
			mockQueries.On("FindUserByEmail", mock.Anything, mock.Anything, tt.email).Return(tt.mockReturn, tt.mockError)

			repo := &UserRepository{queries: mockQueries}
			email, err := user.NewEmail(tt.email)
			require.NoError(t, err)

			userReadModel, hash, err := repo.FindByEmail(context.Background(), email)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, userReadModel)
				assert.Empty(t, hash)
				
				// エラー種別の検証
				if tt.mockError == sql.ErrNoRows {
					assert.True(t, infra.IsKind(err, infra.KindNotFound))
				} else {
					assert.True(t, infra.IsKind(err, infra.KindDBFailure))
				}
			} else {
				assert.NoError(t, err)
				if tt.wantUser {
					assert.NotNil(t, userReadModel)
					assert.Equal(t, tt.email, userReadModel.Email)
					assert.Equal(t, tt.wantHash, hash)
				} else {
					assert.Nil(t, userReadModel)
					assert.Empty(t, hash)
				}
			}

			mockQueries.AssertExpectations(t)
		})
	}
}

func TestFindByID(t *testing.T) {
	testUser := builder.NewUserBuilder().BuildInfra()
	inactiveUser := builder.NewUserBuilder().AsInactive().BuildInfra()
	
	testUserRow := sqlc.FindUserByIDRow{
		ID:        testUser.ID,
		Email:     testUser.Email,
		Role:      testUser.Role,
		CompanyID: testUser.CompanyID,
		IsActive:  testUser.IsActive,
		LastLogin: testUser.LastLogin,
		CreatedAt: testUser.CreatedAt,
		UpdatedAt: testUser.UpdatedAt,
	}

	inactiveUserRow := sqlc.FindUserByIDRow{
		ID:        inactiveUser.ID,
		Email:     inactiveUser.Email,
		Role:      inactiveUser.Role,
		CompanyID: inactiveUser.CompanyID,
		IsActive:  inactiveUser.IsActive,
		LastLogin: inactiveUser.LastLogin,
		CreatedAt: inactiveUser.CreatedAt,
		UpdatedAt: inactiveUser.UpdatedAt,
	}

	tests := []struct {
		name       string
		userID     uuid.UUID
		mockReturn sqlc.FindUserByIDRow
		mockError  error
		wantUser   bool
		wantError  bool
	}{
		{
			name:       "success - active user",
			userID:     testUserRow.ID,
			mockReturn: testUserRow,
			mockError:  nil,
			wantUser:   true,
			wantError:  false,
		},
		{
			name:       "success - inactive user (for validation)",
			userID:     inactiveUserRow.ID,
			mockReturn: inactiveUserRow,
			mockError:  nil,
			wantUser:   true,
			wantError:  false,
		},
		{
			name:       "user not found",
			userID:     uuid.New(),
			mockReturn: sqlc.FindUserByIDRow{},
			mockError:  sql.ErrNoRows,
			wantUser:   false,
			wantError:  true,
		},
		{
			name:       "database error",
			userID:     testUserRow.ID,
			mockReturn: sqlc.FindUserByIDRow{},
			mockError:  assert.AnError,
			wantUser:   false,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockQueries := new(MockQueries)
			mockQueries.On("FindUserByID", mock.Anything, mock.Anything, tt.userID).Return(tt.mockReturn, tt.mockError)

			repo := &UserRepository{queries: mockQueries}

			userReadModel, err := repo.FindByID(context.Background(), tt.userID)

			if tt.wantError {
				assert.Error(t, err)
				assert.Nil(t, userReadModel)
				
				// エラー種別の検証
				if tt.mockError == sql.ErrNoRows {
					assert.True(t, infra.IsKind(err, infra.KindNotFound))
				} else {
					assert.True(t, infra.IsKind(err, infra.KindDBFailure))
				}
			} else {
				assert.NoError(t, err)
				if tt.wantUser {
					assert.NotNil(t, userReadModel)
					assert.Equal(t, tt.userID, userReadModel.ID)
				} else {
					assert.Nil(t, userReadModel)
				}
			}

			mockQueries.AssertExpectations(t)
		})
	}
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
			mockQueries := new(MockQueries)
			mockQueries.On("UpdateUserLastLogin", mock.Anything, mock.Anything, tt.userID).Return(tt.mockError)

			repo := &UserRepository{queries: mockQueries}

			err := repo.UpdateLastLogin(context.Background(), tt.userID)

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
