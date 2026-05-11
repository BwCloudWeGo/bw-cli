package service

import (
	"context"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

func (s *Service) ListDemoOrderItemsByOrderID(ctx context.Context, id int32) ([]*dto.DemoOrderItemDTO, error) {
	if s.queries == nil {
		return nil, model.ErrInvalidOrderReport
	}
	items, err := s.queries.ListDemoOrderItemsByOrderID(ctx, id)
	if err != nil {
		return nil, err
	}
	output := make([]*dto.DemoOrderItemDTO, 0, len(items))
	for _, item := range items {
		output = append(output, dto.FromDemoOrderItem(item))
	}
	return output, nil
}
