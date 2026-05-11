package model

import "time"

// DemoOrderItem is generated from the demo_order_items relation table.
type DemoOrderItem struct {
	ID          int32
	OrderID     int32
	Sku         string
	ProductName string
	Quantity    int32
	UnitPrice   string
	CreatedAt   time.Time
}

var _ = time.Time{}
