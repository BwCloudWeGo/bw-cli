package handler

import (
	"sync"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orderv1 "github.com/BwCloudWeGo/bw-cli/api/gen/order/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

const orderGatewayTargetEnv = "APP_ORDER_GRPC_TARGET"
const orderGatewayDefaultTarget = "127.0.0.1:9100"

// OrderHandler adapts order HTTP endpoints to the generated gRPC client.
type OrderHandler struct {
	target string
	client orderv1.OrderServiceClient
	conn   *grpc.ClientConn
	once   sync.Once
	err    error
	log    *zap.Logger
}

// NewOrderHandler builds a gateway handler with a default target that needs no config changes.
func NewOrderHandler(log *zap.Logger) *OrderHandler {
	if log == nil {
		log = zap.NewNop()
	}
	return &OrderHandler{
		target: gatewayGRPCTarget(orderGatewayTargetEnv, orderGatewayDefaultTarget),
		log:    log,
	}
}

// Create proxies POST /api/v1/orders to CreateOrder.
func (h *OrderHandler) Create(c *gin.Context) {
	var req request.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_grpc_client_error", "order grpc client error", err))
		return
	}
	resp, err := client.CreateOrder(outgoingContext(c), &orderv1.CreateOrderRequest{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order create proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.Created(c, resp)
}

// Get proxies GET /api/v1/orders/:id to GetOrder.
func (h *OrderHandler) Get(c *gin.Context) {
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_grpc_client_error", "order grpc client error", err))
		return
	}
	resp, err := client.GetOrder(outgoingContext(c), &orderv1.GetOrderRequest{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// List proxies GET /api/v1/orders to ListOrders.
func (h *OrderHandler) List(c *gin.Context) {
	var req request.ListOrderRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_grpc_client_error", "order grpc client error", err))
		return
	}
	resp, err := client.ListOrders(outgoingContext(c), &orderv1.ListOrdersRequest{
		Page:     req.Page,
		PageSize: req.PageSize,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}

// Update proxies PUT /api/v1/orders/:id to UpdateOrder.
func (h *OrderHandler) Update(c *gin.Context) {
	var req request.UpdateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_grpc_client_error", "order grpc client error", err))
		return
	}
	resp, err := client.UpdateOrder(outgoingContext(c), &orderv1.UpdateOrderRequest{
		Id:          c.Param("id"),
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order update proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", resp.GetId()))
	httpx.OK(c, resp)
}

// Delete proxies DELETE /api/v1/orders/:id to DeleteOrder.
func (h *OrderHandler) Delete(c *gin.Context) {
	client, err := h.grpcClient()
	if err != nil {
		httpx.Error(c, apperrors.Wrap(apperrors.KindInternal, "order_grpc_client_error", "order grpc client error", err))
		return
	}
	resp, err := client.DeleteOrder(outgoingContext(c), &orderv1.DeleteOrderRequest{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway order delete proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("aggregate_id", c.Param("id")))
	httpx.OK(c, resp)
}

func (h *OrderHandler) grpcClient() (orderv1.OrderServiceClient, error) {
	h.once.Do(func() {
		conn, err := grpc.Dial(h.target, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			h.err = err
			return
		}
		h.conn = conn
		h.client = orderv1.NewOrderServiceClient(conn)
		h.log.Info("gateway order grpc client initialized", zap.String("target", h.target), zap.String("target_env", orderGatewayTargetEnv))
	})
	return h.client, h.err
}
