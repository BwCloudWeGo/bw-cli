package dto

import (
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

// OrderDTO 是 order 用例层返回给 handler 的数据结构。
// 它不包含 gRPC、HTTP 或 Gorm tag，避免协议和数据库细节进入 service 层。
type OrderDTO struct {
	ID          string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
}

// ListOrderDTO 是 order 分页查询的返回结构。
type ListOrderDTO struct {
	Items []*OrderDTO
	Total int64
}

// FromOrder 将 order 领域聚合转换成 service 对外返回的 DTO。
func FromOrder(item *model.Order) *OrderDTO {
	return &OrderDTO{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   formatTime(item.CreatedAt),
		UpdatedAt:   formatTime(item.UpdatedAt),
	}
}

// formatTime 统一时间输出格式；零值时间返回空字符串，避免输出无意义的默认时间。
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
