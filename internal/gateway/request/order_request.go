package request

// CreateOrderRequest is the JSON payload used by POST /api/v1/orders.
type CreateOrderRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateOrderRequest is the JSON payload used by PUT /api/v1/orders/:id.
type UpdateOrderRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// ListOrderRequest is the query string payload used by GET /api/v1/orders.
type ListOrderRequest struct {
	Page     int32 `form:"page"`
	PageSize int32 `form:"page_size"`
}
