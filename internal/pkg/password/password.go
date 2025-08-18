package password

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

var (
	ErrHashingFailed    = errors.New("password hashing failed")
	ErrComparisonFailed = errors.New("password comparison failed")
	ErrInvalidPassword  = errors.New("invalid password")
)

const DefaultCost = bcrypt.DefaultCost

func HashPassword(password string) (string, error) {
	if password == "" {
		return "", ErrInvalidPassword
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), DefaultCost)
	if err != nil {
		return "", ErrHashingFailed
	}

	return string(hashedBytes), nil
}

func ComparePassword(hashedPassword, password string) error {
	if hashedPassword == "" || password == "" {
		return ErrInvalidPassword
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrComparisonFailed
		}
		return err
	}

	return nil
}
