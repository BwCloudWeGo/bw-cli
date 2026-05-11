package handler

import (
	"strconv"
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orderreportv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order_report/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

const orderReportGatewayTargetEnv = "APP_ORDER_REPORT_GRPC_TARGET"
const orderReportGatewayDefaultTarget = "127.0.0.1:9110"

type OrderReportHandler struct {
	target string
	client orderreportv1.OrderReportServiceClient
	conn   *grpc.ClientConn
	once   sync.Once
	err    error
	log    *zap.Logger
}

func NewOrderReportHandler(log *zap.Logger) *OrderReportHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &OrderReportHandler{target: gatewayGRPCTarget(orderReportGatewayTargetEnv, orderReportGatewayDefaultTarget), log: log}
}

func (h *OrderReportHandler) Create(c *gin.Context) {
	var req request.CreateOrderReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_report_grpc_client_error", "order_report grpc client error", err))
		return
	}
	resp, err := client.CreateOrderReport(outgoingContext(c), &orderreportv1.CreateOrderReportRequest{
		CustomerName: req.CustomerName,
		Status:       req.Status,
		TotalAmount:  req.TotalAmount,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order_report create proxied", zap.String("request_id", httpx.RequestID(c)), zap.Any("aggregate_id", resp.GetId()))
	httpx.Created(c, resp)
}

func (h *OrderReportHandler) Get(c *gin.Context) {
	id, err := parseOrderReportID(c.Param("id"))
	if err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_id", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_report_grpc_client_error", "order_report grpc client error", err))
		return
	}
	resp, err := client.GetOrderReport(outgoingContext(c), &orderreportv1.GetOrderReportRequest{Id: id})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

func (h *OrderReportHandler) List(c *gin.Context) {
	var req request.ListOrderReportRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_report_grpc_client_error", "order_report grpc client error", err))
		return
	}
	resp, err := client.ListOrderReports(outgoingContext(c), &orderreportv1.ListOrderReportsRequest{Page: req.Page, PageSize: req.PageSize})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

func (h *OrderReportHandler) Update(c *gin.Context) {
	id, err := parseOrderReportID(c.Param("id"))
	if err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_id", err.Error()))
		return
	}
	var req request.UpdateOrderReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_report_grpc_client_error", "order_report grpc client error", err))
		return
	}
	resp, err := client.UpdateOrderReport(outgoingContext(c), &orderreportv1.UpdateOrderReportRequest{
		Id:           id,
		CustomerName: req.CustomerName,
		Status:       req.Status,
		TotalAmount:  req.TotalAmount,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order_report update proxied", zap.String("request_id", httpx.RequestID(c)), zap.Any("aggregate_id", resp.GetId()))
	httpx.OK(c, resp)
}

func (h *OrderReportHandler) Delete(c *gin.Context) {
	id, err := parseOrderReportID(c.Param("id"))
	if err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_id", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_report_grpc_client_error", "order_report grpc client error", err))
		return
	}
	resp, err := client.DeleteOrderReport(outgoingContext(c), &orderreportv1.DeleteOrderReportRequest{Id: id})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order_report delete proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", c.Param("id")))
	httpx.OK(c, resp)
}

func (h *OrderReportHandler) grpcClient() (orderreportv1.OrderReportServiceClient, error) {
	h.once.Do(func() {
		conn, err := grpc.Dial(h.target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			h.err = err
			return
		}
		h.conn = conn
		h.client = orderreportv1.NewOrderReportServiceClient(conn)
		h.log.Info("gateway order_report grpc client initialized", zap.String("target", h.target), zap.String("target_env", orderReportGatewayTargetEnv))
	})
	return h.client, h.err
}

func parseOrderReportID(value string) (int32, error) {
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil {
		return 0, err
	}
	return int32(parsed), nil
}

var _ = strconv.IntSize
