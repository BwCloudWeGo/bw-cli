package model

import "context"

// Repository 定义 note 业务服务层依赖的通用持久化行为。
type Repository interface {
	Save(ctx context.Context, note *Note) error
	FindByID(ctx context.Context, id string) (*Note, error)
}
