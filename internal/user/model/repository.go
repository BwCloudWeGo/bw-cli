package model

import "context"

// Repository defines persistence behavior required by the user service layer.
type Repository interface {
	Save(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
}
