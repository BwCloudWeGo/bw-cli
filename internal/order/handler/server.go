package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
	"github.com/BwCloudWeGo/bw-cli/internal/order/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server adapts order gRPC requests to service use cases.
type Server struct {
	orderv1.UnimplementedOrderServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer constructs the order gRPC server adapter.
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

// CreateOrder handles the create RPC.
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

// GetOrder handles lookup by id.
func (s *Server) GetOrder(ctx context.Context, req *orderv1.GetOrderRequest) (*orderv1.OrderResponse, error) {
	item, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapOrderError(err)
	}
	return toProto(item), nil
}

// ListOrders handles paginated listing.
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

// UpdateOrder handles updates by id.
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

// DeleteOrder handles deletion by id.
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
	case stderrors.Is(err, model.ErrInvalidOrder):
		return apperrors.InvalidArgument("invalid_order", "invalid order input")
	case stderrors.Is(err, model.ErrOrderNotFound):
		return apperrors.NotFound("order_not_found", "order not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "order_service_error", "order service error", err)
	}
}
