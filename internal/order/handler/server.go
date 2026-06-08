package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/entity"
	"github.com/BwCloudWeGo/bw-cli/internal/order/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server 将 order gRPC 请求适配到 service 用例。
type Server struct {
	orderv1.UnimplementedOrderServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer 创建 order gRPC 服务端适配器。
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

// CreateOrder 处理创建 RPC。
func (s *Server) CreateOrder(ctx context.Context, req *orderv1.CreateOrderRequest) (*orderv1.OrderResponse, error) {
	item, err := s.svc.Create(ctx, dto.CreateCommand{
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toProto(item), nil
}

// GetOrder 处理按 ID 查询。
func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.OrderResponse, error) {
	item, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toProto(item), nil
}

// ListOrders 处理分页列表查询。
func (s *Server) ListOrders(ctx context.Context, req *orderv1.ListOrdersRequest) (*orderv1.ListOrdersResponse, error) {
	list, err := s.svc.List(ctx, dto.ListCommand{
		Page:     req.GetPage(),
		PageSize: req.GetPageSize(),
	})
	if err != nil {
		return nil, mapOrderError(err)
	}
	resp := &orderv1.ListOrdersResponse{
		Items: make([]*orderv1.OrderResponse, 0, len(list.Items)),
		Total: list.Total,
	}
	for _, item := range list.Items {
		resp.Items = append(resp.Items, toProto(item))
	}
	return resp, nil
}

// UpdateOrder 处理按 ID 更新。
func (s *Server) UpdateOrder(ctx context.Context, req *orderv1.UpdateOrderRequest) (*orderv1.OrderResponse, error) {
	item, err := s.svc.Update(ctx, dto.UpdateCommand{
		ID:          req.GetId(),
		Name:        req.GetName(),
		Description: req.GetDescription(),
	})
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toProto(item), nil
}

// DeleteOrder 处理按 ID 删除。
func (s *Server) DeleteOrder(ctx context.Context, req *orderv1.DeleteOrderRequest) (*orderv1.DeleteOrderResponse, error) {
	if err := s.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, mapOrderError(err)
	}
	return &orderv1.DeleteOrderResponse{Success: true}, nil
}

func toProto(item *dto.OrderDTO) *orderv1.OrderResponse {
	return &orderv1.OrderResponse{
		Id:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}
}

func mapOrderError(err error) error {
	switch {
	case stderrors.Is(err, entity.ErrInvalidOrder):
		return apperrors.InvalidArgument("invalid_order", "invalid order input")
	case stderrors.Is(err, entity.ErrOrderNotFound):
		return apperrors.NotFound("order_not_found", "order not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "order_service_error", "order service error", err)
	}
}
