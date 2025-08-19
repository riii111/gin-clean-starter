package user

import (
	"time"

	"github.com/google/uuid"
)

// User entity. Currently used for auth only.
// TODO: Add user management APIs in future phases.
type User struct {
	id           uuid.UUID
	email        Email
	passwordHash string
	role         Role
	companyID    *uuid.UUID
	lastLogin    *time.Time
	isActive     bool
	createdAt    time.Time
	updatedAt    time.Time
}

func NewUser(email Email, passwordHash string, role Role, companyID *uuid.UUID) *User {
	return &User{
		id:           uuid.New(),
		email:        email,
		passwordHash: passwordHash,
		role:         role,
		companyID:    companyID,
		isActive:     true,
	}
}

func (u *User) ID() uuid.UUID         { return u.id }
func (u *User) Email() Email          { return u.email }
func (u *User) PasswordHash() string  { return u.passwordHash }
func (u *User) Role() Role            { return u.role }
func (u *User) CompanyID() *uuid.UUID { return u.companyID }
func (u *User) LastLogin() *time.Time { return u.lastLogin }
func (u *User) IsActive() bool        { return u.isActive }
func (u *User) CreatedAt() time.Time  { return u.createdAt }
func (u *User) UpdatedAt() time.Time  { return u.updatedAt }
