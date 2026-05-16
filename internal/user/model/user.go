package model

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrUserNotFound         = errors.New("user not found")
	ErrAccountAlreadyExists = errors.New("account already exists")
	ErrInvalidCredentials   = errors.New("invalid credentials")
	ErrInvalidUser          = errors.New("invalid user")
)

// User is the user aggregate used by the user service.
type User struct {
	ID           string
	Account      string
	DisplayName  string
	PasswordHash string
	PasswordSalt string
	Sex          bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser validates input and creates a user aggregate with a normalized account.
func NewUser(account string, displayName string, passwordHash string) (*User, error) {
	account = NormalizeAccount(account)
	displayName = strings.TrimSpace(displayName)
	if account == "" || displayName == "" || passwordHash == "" {
		return nil, ErrInvalidUser
	}

	now := time.Now().UTC()
	return &User{
		Account:      account,
		DisplayName:  displayName,
		PasswordHash: passwordHash,
		CreatedAt:    now,
		UpdatedAt:    now,
	}, nil
}

// NormalizeAccount trims and lowercases account names for unique lookup.
func NormalizeAccount(account string) string {
	return strings.TrimSpace(strings.ToLower(account))
}
