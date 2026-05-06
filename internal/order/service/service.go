package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
)

// Service orchestrates order use cases.
type Service struct {
	repo model.Repository
	log  *zap.Logger
}

// NewService constructs the order use-case service.
func NewService(repo model.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	return &Service{repo: repo, log: log}
}

// Create creates a order record.
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) (*dto.OrderDTO, error) {
	item, err := model.NewOrder(cmd.Name, cmd.Description)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("order created", zap.String("aggregate_id", item.ID), zap.String("use_case", "CreateOrder"))
	return dto.FromOrder(item), nil
}

// Get returns one order record by id.
func (s *Service) Get(ctx context.Context, id string) (*dto.OrderDTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.FromOrder(item), nil
}

// List returns paginated order records.
func (s *Service) List(ctx context.Context, cmd dto.ListCommand) (*dto.ListOrderDTO, error) {
	offset, limit := normalizePagination(cmd.Page, cmd.PageSize)
	items, total, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	output := &dto.ListOrderDTO{Items: make([]*dto.OrderDTO, 0, len(items)), Total: total}
	for _, item := range items {
		output.Items = append(output.Items, dto.FromOrder(item))
	}
	return output, nil
}

// Update changes one order record by id.
func (s *Service) Update(ctx context.Context, cmd dto.UpdateCommand) (*dto.OrderDTO, error) {
	item, err := s.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := item.Update(cmd.Name, cmd.Description); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("order updated", zap.String("aggregate_id", item.ID), zap.String("use_case", "UpdateOrder"))
	return dto.FromOrder(item), nil
}

// Delete removes one order record by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("order deleted", zap.String("aggregate_id", id), zap.String("use_case", "DeleteOrder"))
	return nil
}

func normalizePagination(page int32, pageSize int32) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return int((page - 1) * pageSize), int(pageSize)
}
