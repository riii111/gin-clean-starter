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

func (p Password) Value() string {
	return p.value
}
