package request

// CreateOrderRequest 是 POST /api/v1/orders 使用的 JSON 载荷。
type CreateOrderRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// UpdateOrderRequest 是 PUT /api/v1/orders/:id 使用的 JSON 载荷。
type UpdateOrderRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

// ListOrderRequest 是 GET /api/v1/orders 使用的查询参数载荷。
type ListOrderRequest struct {
	Page     int32 `form:"page"`
	PageSize int32 `form:"page_size"`
}
