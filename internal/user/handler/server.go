package handler

import (
	"context"
	stderrors "errors"

	"go.uber.org/zap"

	userv1 "github.com/BwCloudWeGo/bw-cli/api/gen/user/v1"
	"github.com/BwCloudWeGo/bw-cli/internal/user/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
	"github.com/BwCloudWeGo/bw-cli/internal/user/service"
	apperrors "github.com/BwCloudWeGo/bw-cli/pkg/errors"
)

// Server adapts user gRPC requests to user service use cases.
type Server struct {
	userv1.UnimplementedUserServiceServer
	svc *service.Service
	log *zap.Logger
}

// NewServer constructs the user gRPC server adapter.
func NewServer(svc *service.Service, log *zap.Logger) *Server {
	return &Server{svc: svc, log: log}
}

// Register handles the user registration RPC.
func (s *Server) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.UserResponse, error) {
	user, err := s.svc.Register(ctx, dto.RegisterCommand{
		Email:       req.GetEmail(),
		DisplayName: req.GetDisplayName(),
		Password:    req.GetPassword(),
	})
	if err != nil {
		return nil, mapUserError(err)
	}
	s.log.Info("user registered", zap.String("aggregate_id", user.ID), zap.String("use_case", "Register"))
	return toProto(user), nil
}

// Login handles the user login RPC.
func (s *Server) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.UserResponse, error) {
	user, err := s.svc.Login(ctx, dto.LoginCommand{
		Email:    req.GetEmail(),
		Password: req.GetPassword(),
	})
	if err != nil {
		return nil, mapUserError(err)
	}
	s.log.Info("user logged in", zap.String("aggregate_id", user.ID), zap.String("use_case", "Login"))
	return toProto(user), nil
}

// GetUser handles user profile lookup by id.
func (s *Server) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.UserResponse, error) {
	user, err := s.svc.GetUser(ctx, req.GetId())
	if err != nil {
		return nil, mapUserError(err)
	}
	return toProto(user), nil
}

func toProto(user *dto.UserDTO) *userv1.UserResponse {
	return &userv1.UserResponse{
		Id:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}
}

func mapUserError(err error) error {
	switch {
	case stderrors.Is(err, model.ErrInvalidUser):
		return apperrors.InvalidArgument("invalid_user", "invalid user input")
	case stderrors.Is(err, model.ErrEmailAlreadyExists):
		return apperrors.Conflict("email_already_exists", "email already exists")
	case stderrors.Is(err, model.ErrUserNotFound):
		return apperrors.NotFound("user_not_found", "user not found")
	case stderrors.Is(err, model.ErrInvalidCredentials):
		return apperrors.Unauthorized("invalid_credentials", "invalid credentials")
	default:
		return apperrors.Wrap(apperrors.KindInternal, "user_service_error", "user service error", err)
	}
}
