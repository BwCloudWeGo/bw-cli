package service

import (
	"context"
	"errors"
	"strings"

	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
)

// PasswordHasher hides password hashing implementation details from business use cases.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hash string, password string) bool
}

// Service orchestrates user use cases.
type Service struct {
	repo   model.Repository
	hasher PasswordHasher
}

// NewService constructs the user use-case service.
func NewService(repo model.Repository, hasher PasswordHasher) *Service {
	return &Service{repo: repo, hasher: hasher}
}

// RegisterCommand contains validated input for registering a user.
type RegisterCommand struct {
	Email       string
	DisplayName string
	Password    string
}

// LoginCommand contains validated input for logging in a user.
type LoginCommand struct {
	Email    string
	Password string
}

// UserDTO is the public user data returned by use cases.
type UserDTO struct {
	ID          string
	Email       string
	DisplayName string
}

// Register creates a new user after checking email uniqueness.
func (s *Service) Register(ctx context.Context, cmd RegisterCommand) (*UserDTO, error) {
	if strings.TrimSpace(cmd.Password) == "" {
		return nil, model.ErrInvalidUser
	}
	if _, err := s.repo.FindByEmail(ctx, strings.TrimSpace(strings.ToLower(cmd.Email))); err == nil {
		return nil, model.ErrEmailAlreadyExists
	} else if !errors.Is(err, model.ErrUserNotFound) {
		return nil, err
	}

	hash, err := s.hasher.Hash(cmd.Password)
	if err != nil {
		return nil, err
	}
	user, err := model.NewUser(cmd.Email, cmd.DisplayName, hash)
	if err != nil {
		return nil, err
	}
	if err := s.repo.Save(ctx, user); err != nil {
		return nil, err
	}
	return toDTO(user), nil
}

// Login verifies credentials and returns the matching user.
func (s *Service) Login(ctx context.Context, cmd LoginCommand) (*UserDTO, error) {
	user, err := s.repo.FindByEmail(ctx, strings.TrimSpace(strings.ToLower(cmd.Email)))
	if err != nil {
		if errors.Is(err, model.ErrUserNotFound) {
			return nil, model.ErrInvalidCredentials
		}
		return nil, err
	}
	if !s.hasher.Verify(user.PasswordHash, cmd.Password) {
		return nil, model.ErrInvalidCredentials
	}
	return toDTO(user), nil
}

// GetUser returns one user by id.
func (s *Service) GetUser(ctx context.Context, id string) (*UserDTO, error) {
	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toDTO(user), nil
}

func toDTO(user *model.User) *UserDTO {
	return &UserDTO{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
	}
}
