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

const (
	DefaultCost  = bcrypt.DefaultCost
	MaxBcryptLen = 72 // bcrypt truncates passwords longer than 72 bytes
)

func HashPassword(password string) (string, error) {
	return HashPasswordWithCost(password, DefaultCost)
}

func HashPasswordWithCost(password string, cost int) (string, error) {
	if password == "" || len(password) > MaxBcryptLen {
		return "", ErrInvalidPassword
	}

	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", ErrHashingFailed
	}

	return string(hashedBytes), nil
}

func ComparePassword(hashedPassword, password string) error {
	if hashedPassword == "" || password == "" || len(password) > MaxBcryptLen {
		return ErrInvalidPassword
	}

	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	if err != nil {
		// Normalize all bcrypt errors to prevent information leakage
		return ErrComparisonFailed
	}

	return nil
}

func NeedsRehash(hashedPassword string, desiredCost int) bool {
	if hashedPassword == "" {
		return false
	}

	cost, err := bcrypt.Cost([]byte(hashedPassword))
	if err != nil {
		return true
	}

	return cost != desiredCost
}
