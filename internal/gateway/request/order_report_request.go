package request

// CreateOrderReportRequest is the JSON payload used by POST /api/v1/demo_orders.
type CreateOrderReportRequest struct {
	CustomerName string `json:"customer_name" binding:"required"`
	Status       string `json:"status" binding:"required"`
	TotalAmount  string `json:"total_amount" binding:"required"`
}

// UpdateOrderReportRequest is the JSON payload used by PUT /api/v1/demo_orders/:id.
type UpdateOrderReportRequest struct {
	CustomerName string `json:"customer_name" binding:"required"`
	Status       string `json:"status" binding:"required"`
	TotalAmount  string `json:"total_amount" binding:"required"`
}

// ListOrderReportRequest is the query string payload used by GET /api/v1/demo_orders.
type ListOrderReportRequest struct {
	Page     int32 `form:"page"`
	PageSize int32 `form:"page_size"`
}
