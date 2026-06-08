package entity

import "context"

// Repository 定义 user 服务层需要的持久化行为。
type Repository interface {
	Save(ctx context.Context, user *User) error
	FindByID(ctx context.Context, id string) (*User, error)
	FindByAccount(ctx context.Context, account string) (*User, error)
}
