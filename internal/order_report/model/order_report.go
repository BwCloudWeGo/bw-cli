package model

import (
	"errors"
	"time"
)

var (
	ErrOrderReportNotFound = errors.New("order_report not found")
	ErrInvalidOrderReport  = errors.New("invalid order_report")
)

// OrderReport is the aggregate root generated from the demo_orders table.
type OrderReport struct {
	ID           int32
	CustomerName string
	Status       string
	TotalAmount  string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewOrderReport creates an aggregate from writable table fields.
func NewOrderReport(customerName string, status string, totalAmount string) (*OrderReport, error) {
	return &OrderReport{
		CustomerName: customerName,
		Status:       status,
		TotalAmount:  totalAmount,
	}, nil
}

// Update changes writable fields while keeping readonly table fields untouched.
func (item *OrderReport) Update(customerName string, status string, totalAmount string) error {
	if item == nil {
		return ErrInvalidOrderReport
	}
	item.CustomerName = customerName
	item.Status = status
	item.TotalAmount = totalAmount
	return nil
}
