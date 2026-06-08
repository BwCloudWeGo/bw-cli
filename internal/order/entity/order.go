package entity

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrOrderNotFound = errors.New("order not found")
	ErrInvalidOrder  = errors.New("invalid order")
)

// Order 是 order 业务服务的聚合根。
// 业务明确后，请将 Name 和 Description 替换为真实业务字段。
type Order struct {
	ID          string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewOrder 校验输入，并创建带框架管理身份字段的聚合。
func NewOrder(name string, description string) (*Order, error) {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if name == "" {
		return nil, ErrInvalidOrder
	}
	now := time.Now().UTC()
	return &Order{
		ID:          uuid.NewString(),
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// Update 修改可变字段，并把校验保留在业务实体内部。
func (item *Order) Update(name string, description string) error {
	name = strings.TrimSpace(name)
	description = strings.TrimSpace(description)
	if item == nil || item.ID == "" || name == "" {
		return ErrInvalidOrder
	}
	item.Name = name
	item.Description = description
	item.UpdatedAt = time.Now().UTC()
	return nil
}
