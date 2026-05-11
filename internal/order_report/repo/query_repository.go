package repo

import (
	"context"
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

// DemoOrderItemModel is the Gorm persistence model for the demo_order_items table.
type DemoOrderItemModel struct {
	ID          int32     `gorm:"column:id;primaryKey"`
	OrderID     int32     `gorm:"column:order_id"`
	Sku         string    `gorm:"column:sku"`
	ProductName string    `gorm:"column:product_name"`
	Quantity    int32     `gorm:"column:quantity"`
	UnitPrice   string    `gorm:"column:unit_price"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (DemoOrderItemModel) TableName() string {
	return "demo_order_items"
}

func (r *GormRepository) ListDemoOrderItemsByOrderID(ctx context.Context, id int32) ([]*model.DemoOrderItem, error) {
	var records []DemoOrderItemModel
	if err := r.db.WithContext(ctx).Where("order_id = ?", id).Find(&records).Error; err != nil {
		return nil, err
	}
	items := make([]*model.DemoOrderItem, 0, len(records))
	for i := range records {
		items = append(items, toDemoOrderItemDomain(&records[i]))
	}
	return items, nil
}

func toDemoOrderItemDomain(record *DemoOrderItemModel) *model.DemoOrderItem {
	return &model.DemoOrderItem{
		ID:          record.ID,
		OrderID:     record.OrderID,
		Sku:         record.Sku,
		ProductName: record.ProductName,
		Quantity:    record.Quantity,
		UnitPrice:   record.UnitPrice,
		CreatedAt:   record.CreatedAt,
	}
}

var _ = time.Time{}
var _ model.QueryRepository = (*GormRepository)(nil)
