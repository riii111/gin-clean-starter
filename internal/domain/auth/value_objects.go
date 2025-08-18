package auth

import (
	"errors"

	"gin-clean-starter/internal/domain/user"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
)

type Credentials struct {
	email    user.Email
	password user.Password
}

func NewCredentials(emailStr, passwordStr string) (Credentials, error) {
	email, err := user.NewEmail(emailStr)
	if err != nil {
		return Credentials{}, err
	}

	password, err := user.NewPassword(passwordStr)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		email:    email,
		password: password,
	}, nil
}

func (c Credentials) Email() user.Email {
	return c.email
}

func (c Credentials) Password() user.Password {
	return c.password
}
