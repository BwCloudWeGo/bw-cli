package model

import "time"

// UserModel is the Gorm persistence model for the xls_user table.
type UserModel struct {
	ID           int64  `gorm:"primaryKey;column:id;autoIncrement"`
	Account      string `gorm:"uniqueIndex;column:account;size:64;not null"`
	DisplayName  string `gorm:"column:name;size:20"`
	Sex          bool   `gorm:"column:sex"`
	PasswordSalt string `gorm:"column:password_salt;size:64;not null"`
	PasswordHash string `gorm:"column:password_hash;size:255;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (UserModel) TableName() string {
	return "xls_user"
}
