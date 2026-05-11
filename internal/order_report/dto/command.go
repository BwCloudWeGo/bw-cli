package dto

// CreateCommand contains writable input for creating a demo_orders record.
type CreateCommand struct {
	CustomerName string
	Status       string
	TotalAmount  string
}

// UpdateCommand contains primary key and writable input for updating a demo_orders record.
type UpdateCommand struct {
	ID           int32
	CustomerName string
	Status       string
	TotalAmount  string
}

// ListCommand contains pagination input for listing demo_orders records.
type ListCommand struct {
	Page     int32
	PageSize int32
}
