//go:build unit || e2e

package builder

import (
	"time"

	"gin-clean-starter/internal/domain/user"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type UserBuilder struct {
	Email        string
	PasswordHash string
	Role         string
	CompanyID    *uuid.UUID
	IsActive     bool
}

func NewUserBuilder() *UserBuilder {
	companyID := uuid.New()
	return &UserBuilder{
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
		Role:         "admin",
		CompanyID:    &companyID,
		IsActive:     true,
	}
}

func (u *UserBuilder) With(mutate func(*UserBuilder)) *UserBuilder {
	mutate(u)
	return u
}

// Build methods
func (u *UserBuilder) BuildDomain() (*user.User, error) {
	email, err := user.NewEmail(u.Email)
	if err != nil {
		return nil, err
	}

	role, err := user.NewRole(u.Role)
	if err != nil {
		return nil, err
	}

	return user.NewUser(email, u.PasswordHash, role, u.CompanyID), nil
}

func (u *UserBuilder) BuildInfra() sqlc.Users {
	now := time.Now()
	var companyID pgtype.UUID
	if u.CompanyID != nil {
		companyID = pgtype.UUID{Bytes: *u.CompanyID, Valid: true}
	}

	return sqlc.Users{
		ID:           uuid.New(),
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Role:         u.Role,
		CompanyID:    companyID,
		LastLogin:    pgtype.Timestamptz{},
		IsActive:     u.IsActive,
		CreatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:    pgtype.Timestamptz{Time: now, Valid: true},
	}
}

func (u *UserBuilder) BuildReadModel() *queries.AuthorizedUserView {
	return &queries.AuthorizedUserView{
		ID:        uuid.New(),
		Email:     u.Email,
		Role:      u.Role,
		CompanyID: u.CompanyID,
		IsActive:  u.IsActive,
	}
}

// Fluent builder methods
func (u *UserBuilder) WithEmail(email string) *UserBuilder {
	u.Email = email
	return u
}

func (u *UserBuilder) WithRole(role string) *UserBuilder {
	u.Role = role
	return u
}

func (u *UserBuilder) WithPasswordHash(hash string) *UserBuilder {
	u.PasswordHash = hash
	return u
}

func (u *UserBuilder) WithCompanyID(companyID *uuid.UUID) *UserBuilder {
	u.CompanyID = companyID
	return u
}

func (u *UserBuilder) WithoutCompany() *UserBuilder {
	u.CompanyID = nil
	return u
}

func (u *UserBuilder) AsInactive() *UserBuilder {
	u.IsActive = false
	return u
}
