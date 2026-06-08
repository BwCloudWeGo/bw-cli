package entity

import "context"

// Repository 定义 order 服务层需要的持久化行为。
type Repository interface {
	Save(ctx context.Context, item *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, offset int, limit int) ([]*Order, int64, error)
	Delete(ctx context.Context, id string) error
}
