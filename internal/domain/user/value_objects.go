package user

import (
	"errors"
	"regexp"
	"strings"
)

var (
	ErrInvalidEmail    = errors.New("invalid email format")
	ErrInvalidRole     = errors.New("invalid role")
	ErrPasswordTooWeak = errors.New("password must be at least 8 characters long")
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

type Email struct {
	value string
}

func NewEmail(s string) (Email, error) {
	s = strings.TrimSpace(s)
	if !emailRegex.MatchString(s) {
		return Email{}, ErrInvalidEmail
	}
	return Email{value: s}, nil
}

func (e Email) Value() string {
	return e.value
}

type Password struct {
	value string
}

func NewPassword(s string) (Password, error) {
	if len(s) < 8 {
		return Password{}, ErrPasswordTooWeak
	}
	return Password{value: s}, nil
}

type Credentials struct {
	email    Email
	password Password
}

func NewCredentials(emailStr, passwordStr string) (Credentials, error) {
	email, err := NewEmail(emailStr)
	if err != nil {
		return Credentials{}, err
	}

	password, err := NewPassword(passwordStr)
	if err != nil {
		return Credentials{}, err
	}

	return Credentials{
		email:    email,
		password: password,
	}, nil
}

func (c Credentials) Email() Email {
	return c.email
}

func (c Credentials) Password() Password {
	return c.password
}

func (p Password) Value() string {
	return p.value
}
