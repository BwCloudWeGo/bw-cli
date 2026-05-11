package dto

import (
	"time"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

// OrderReportDTO is returned by use cases and converted by handlers.
type OrderReportDTO struct {
	ID           int32
	CustomerName string
	Status       string
	TotalAmount  string
	CreatedAt    string
	UpdatedAt    string
}

// ListOrderReportDTO contains paginated list output.
type ListOrderReportDTO struct {
	Items []*OrderReportDTO
	Total int64
}

// FromOrderReport converts a order_report aggregate into the service response DTO.
func FromOrderReport(item *model.OrderReport) *OrderReportDTO {
	return &OrderReportDTO{
		ID:           item.ID,
		CustomerName: item.CustomerName,
		Status:       item.Status,
		TotalAmount:  item.TotalAmount,
		CreatedAt:    formatTime(item.CreatedAt),
		UpdatedAt:    formatTime(item.UpdatedAt),
	}
}

// formatTime keeps zero time empty and serializes real values in a stable API format.
func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339Nano)
}
