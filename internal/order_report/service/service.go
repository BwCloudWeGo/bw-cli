package service

import (
	"context"

	"go.uber.org/zap"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
)

// Service orchestrates order_report use cases.
type Service struct {
	repo    model.Repository
	queries model.QueryRepository
	log     *zap.Logger
}

// NewService constructs the order_report use-case service.
func NewService(repo model.Repository, log *zap.Logger) *Service {
	if log == nil {
		log = zap.NewNop()
	}
	var queries model.QueryRepository
	if queryRepo, ok := repo.(model.QueryRepository); ok {
		queries = queryRepo
	}
	return &Service{repo: repo, queries: queries, log: log}
}

// Create creates a demo_orders record.
func (s *Service) Create(ctx context.Context, cmd dto.CreateCommand) (*dto.OrderReportDTO, error) {
	item, err := model.NewOrderReport(cmd.CustomerName, cmd.Status, cmd.TotalAmount)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("order_report created", zap.Any("aggregate_id", item.ID), zap.String("use_case", "CreateOrderReport"))
	return dto.FromOrderReport(item), nil
}

// Get returns one demo_orders record by id.
func (s *Service) Get(ctx context.Context, id int32) (*dto.OrderReportDTO, error) {
	item, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return dto.FromOrderReport(item), nil
}

// List returns paginated demo_orders records.
func (s *Service) List(ctx context.Context, cmd dto.ListCommand) (*dto.ListOrderReportDTO, error) {
	offset, limit := normalizePagination(cmd.Page, cmd.PageSize)
	items, total, err := s.repo.List(ctx, offset, limit)
	if err != nil {
		return nil, err
	}
	output := &dto.ListOrderReportDTO{Items: make([]*dto.OrderReportDTO, 0, len(items)), Total: total}
	for _, item := range items {
		output.Items = append(output.Items, dto.FromOrderReport(item))
	}
	return output, nil
}

// Update changes one demo_orders record by id.
func (s *Service) Update(ctx context.Context, cmd dto.UpdateCommand) (*dto.OrderReportDTO, error) {
	item, err := s.repo.FindByID(ctx, cmd.ID)
	if err != nil {
		return nil, err
	}
	if err := item.Update(cmd.CustomerName, cmd.Status, cmd.TotalAmount); err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, item); err != nil {
		return nil, err
	}
	s.log.Info("order_report updated", zap.Any("aggregate_id", item.ID), zap.String("use_case", "UpdateOrderReport"))
	return dto.FromOrderReport(item), nil
}

// Delete removes one demo_orders record by id.
func (s *Service) Delete(ctx context.Context, id int32) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}
	s.log.Info("order_report deleted", zap.Any("aggregate_id", id), zap.String("use_case", "DeleteOrderReport"))
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
