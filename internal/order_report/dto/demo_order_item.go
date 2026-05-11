package dto

import (
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

// DemoOrderItemDTO is returned by relation query use cases.
type DemoOrderItemDTO struct {
	ID          int32
	OrderID     int32
	Sku         string
	ProductName string
	Quantity    int32
	UnitPrice   string
	CreatedAt   string
}

// FromDemoOrderItem converts a relation model into a DTO.
func FromDemoOrderItem(item *model.DemoOrderItem) *DemoOrderItemDTO {
	return &DemoOrderItemDTO{
		ID:          item.ID,
		OrderID:     item.OrderID,
		Sku:         item.Sku,
		ProductName: item.ProductName,
		Quantity:    item.Quantity,
		UnitPrice:   item.UnitPrice,
		CreatedAt:   formatTime(item.CreatedAt),
	}
}

var _ = time.RFC3339Nano
