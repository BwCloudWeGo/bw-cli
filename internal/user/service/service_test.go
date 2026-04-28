package service_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
	"github.com/BwCloudWeGo/bw-cli/internal/user/service"
)

type memoryUserRepo struct {
	byID    map[string]*model.User
	byEmail map[string]*model.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		byID:    map[string]*model.User{},
		byEmail: map[string]*model.User{},
	}
}

func (r *memoryUserRepo) Save(_ context.Context, user *model.User) error {
	r.byID[user.ID] = user
	r.byEmail[user.Email] = user
	return nil
}

func (r *memoryUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	user, ok := r.byID[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return user, nil
}

func (r *memoryUserRepo) FindByEmail(_ context.Context, email string) (*model.User, error) {
	user, ok := r.byEmail[email]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return user, nil
}

type plainHasher struct{}

func (plainHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func (plainHasher) Verify(hash string, password string) bool {
	return hash == "hashed:"+password
}

func TestRegisterCreatesUserAndRejectsDuplicateEmail(t *testing.T) {
	svc := service.NewService(newMemoryUserRepo(), plainHasher{})

	created, err := svc.Register(context.Background(), service.RegisterCommand{
		Email:       "ada@example.com",
		DisplayName: "Ada",
		Password:    "secret123",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "ada@example.com", created.Email)
	require.Equal(t, "Ada", created.DisplayName)

	_, err = svc.Register(context.Background(), service.RegisterCommand{
		Email:       "ada@example.com",
		DisplayName: "Ada Again",
		Password:    "secret123",
	})
	require.ErrorIs(t, err, model.ErrEmailAlreadyExists)
}

func TestLoginValidatesPassword(t *testing.T) {
	svc := service.NewService(newMemoryUserRepo(), plainHasher{})
	_, err := svc.Register(context.Background(), service.RegisterCommand{
		Email:       "grace@example.com",
		DisplayName: "Grace",
		Password:    "secret123",
	})
	require.NoError(t, err)

	user, err := svc.Login(context.Background(), service.LoginCommand{
		Email:    "grace@example.com",
		Password: "secret123",
	})
	require.NoError(t, err)
	require.Equal(t, "grace@example.com", user.Email)

	_, err = svc.Login(context.Background(), service.LoginCommand{
		Email:    "grace@example.com",
		Password: "wrong",
	})
	require.ErrorIs(t, err, model.ErrInvalidCredentials)
}
