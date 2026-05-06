package model

import "context"

// Repository defines persistence behavior required by the order service layer.
type Repository interface {
	Save(ctx context.Context, item *Order) error
	FindByID(ctx context.Context, id string) (*Order, error)
	List(ctx context.Context, offset int, limit int) ([]*Order, int64, error)
	Delete(ctx context.Context, id string) error
}
