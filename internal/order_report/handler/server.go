package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	orderreportv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order_report/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server adapts order_report gRPC requests to service use cases.
type Server struct {
	orderreportv1.UnimplementedOrderReportServiceServer
	svc *service.Service
	log *zap.Logger
}

func NewServer(svc *service.Service, log *zap.Logger) *Server {
	if log == nil {
		log = zap.NewNop()
	}
	return &Server{svc: svc, log: log}
}

func (s *Server) CreateOrderReport(ctx context.Context, req *orderreportv1.CreateOrderReportRequest) (*orderreportv1.OrderReportResponse, error) {
	item, err := s.svc.Create(ctx, dto.CreateCommand{
		CustomerName: req.GetCustomerName(),
		Status:       req.GetStatus(),
		TotalAmount:  req.GetTotalAmount(),
	})
	if err != nil {
		return nil, mapOrderReportError(err)
	}
	return toProto(item), nil
}

func (s *Server) GetOrderReport(ctx context.Context, req *orderreportv1.GetOrderReportRequest) (*orderreportv1.OrderReportResponse, error) {
	item, err := s.svc.Get(ctx, req.GetId())
	if err != nil {
		return nil, mapOrderReportError(err)
	}
	return toProto(item), nil
}

func (s *Server) ListOrderReports(ctx context.Context, req *orderreportv1.ListOrderReportsRequest) (*orderreportv1.ListOrderReportsResponse, error) {
	list, err := s.svc.List(ctx, dto.ListCommand{Page: req.GetPage(), PageSize: req.GetPageSize()})
	if err != nil {
		return nil, mapOrderReportError(err)
	}
	resp := &orderreportv1.ListOrderReportsResponse{Items: make([]*orderreportv1.OrderReportResponse, 0, len(list.Items)), Total: list.Total}
	for _, item := range list.Items {
		resp.Items = append(resp.Items, toProto(item))
	}
	return resp, nil
}

func (s *Server) UpdateOrderReport(ctx context.Context, req *orderreportv1.UpdateOrderReportRequest) (*orderreportv1.OrderReportResponse, error) {
	item, err := s.svc.Update(ctx, dto.UpdateCommand{
		ID:           req.GetId(),
		CustomerName: req.GetCustomerName(),
		Status:       req.GetStatus(),
		TotalAmount:  req.GetTotalAmount(),
	})
	if err != nil {
		return nil, mapOrderReportError(err)
	}
	return toProto(item), nil
}

func (s *Server) DeleteOrderReport(ctx context.Context, req *orderreportv1.DeleteOrderReportRequest) (*orderreportv1.DeleteOrderReportResponse, error) {
	if err := s.svc.Delete(ctx, req.GetId()); err != nil {
		return nil, mapOrderReportError(err)
	}
	return &orderreportv1.DeleteOrderReportResponse{Success: true}, nil
}

func toProto(item *dto.OrderReportDTO) *orderreportv1.OrderReportResponse {
	return &orderreportv1.OrderReportResponse{
		Id:           item.ID,
		CustomerName: item.CustomerName,
		Status:       item.Status,
		TotalAmount:  item.TotalAmount,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}

func (s *Server) ListDemoOrderItemsByOrderID(ctx context.Context, req *orderreportv1.ListDemoOrderItemsByOrderIDRequest) (*orderreportv1.ListDemoOrderItemsByOrderIDResponse, error) {
	items, err := s.svc.ListDemoOrderItemsByOrderID(ctx, req.GetId())
	if err != nil {
		return nil, mapOrderReportError(err)
	}
	resp := &orderreportv1.ListDemoOrderItemsByOrderIDResponse{Items: make([]*orderreportv1.DemoOrderItemResponse, 0, len(items))}
	for _, item := range items {
		resp.Items = append(resp.Items, toDemoOrderItemProto(item))
	}
	return resp, nil
}

func toDemoOrderItemProto(item *dto.DemoOrderItemDTO) *orderreportv1.DemoOrderItemResponse {
	return &orderreportv1.DemoOrderItemResponse{
		Id:          item.ID,
		OrderId:     item.OrderID,
		Sku:         item.Sku,
		ProductName: item.ProductName,
		Quantity:    item.Quantity,
		UnitPrice:   item.UnitPrice,
		CreatedAt:   item.CreatedAt,
	}
}

func mapOrderReportError(err error) error {
	switch {
	case stderrors.Is(err, model.ErrInvalidOrderReport):
		return apperrors.InvalidArgument("invalid_order_report", "invalid order_report input")
	case stderrors.Is(err, model.ErrOrderReportNotFound):
		return apperrors.NotFound("order_report_not_found", "order_report not found")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "order_report_service_error", "order_report service error", err)
	}
}
