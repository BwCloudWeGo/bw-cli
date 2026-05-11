package model

import "context"

// Repository defines persistence behavior required by the order_report service layer.
type Repository interface {
	Save(ctx context.Context, item *OrderReport) error
	FindByID(ctx context.Context, id int32) (*OrderReport, error)
	List(ctx context.Context, offset int, limit int) ([]*OrderReport, int64, error)
	Delete(ctx context.Context, id int32) error
}

// QueryRepository defines relation query behavior required by the service layer.
type QueryRepository interface {
	ListDemoOrderItemsByOrderID(ctx context.Context, id int32) ([]*DemoOrderItem, error)
}
