package entity

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

// User 是 user 服务使用的用户聚合。
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

// NewUser 校验输入，并创建带规范化账号的用户聚合。
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

// NormalizeAccount 去除账号首尾空白并转小写，用于唯一查询。
func NormalizeAccount(account string) string {
	return strings.TrimSpace(strings.ToLower(account))
}
