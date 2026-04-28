package handler

import (
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/gateway/request"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
	"github.com/BwCloudWeGo/bw-cli/pkg/httpx"
)

// UserHandler adapts user HTTP endpoints to the internal user gRPC client.
type UserHandler struct {
	client userv1.UserServiceClient
	log    *zap.Logger
}

// NewUserHandler wires the user gRPC client into HTTP handler methods.
func NewUserHandler(client userv1.UserServiceClient, log *zap.Logger) *UserHandler {
	return &UserHandler{client: client, log: log}
}

// Register handles user registration requests from the HTTP gateway.
func (h *UserHandler) Register(c *gin.Context) {
	var req request.RegisterUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.Register(outgoingContext(c), &userv1.RegisterRequest{
		Email:       req.Email,
		DisplayName: req.DisplayName,
		Password:    req.Password,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway user register proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("user_id", resp.GetId()))
	httpx.Created(c, resp)
}

// Login handles user login requests from the HTTP gateway.
func (h *UserHandler) Login(c *gin.Context) {
	var req request.LoginUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.Error(c, apperrors.InvalidArgument("invalid_request", err.Error()))
		return
	}
	resp, err := h.client.Login(outgoingContext(c), &userv1.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	h.log.Info("gateway user login proxied", zap.String("request_id", httpx.RequestID(c)), zap.String("user_id", resp.GetId()))
	httpx.OK(c, resp)
}

// GetUser proxies user profile lookup by id.
func (h *UserHandler) GetUser(c *gin.Context) {
	resp, err := h.client.GetUser(outgoingContext(c), &userv1.GetUserRequest{Id: c.Param("id")})
	if err != nil {
		httpx.Error(c, apperrors.FromGRPC(err))
		return
	}
	httpx.OK(c, resp)
}
