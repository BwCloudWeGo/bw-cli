package service_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/BwCloudWeGo/bw-cli/internal/user/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/user/model"
	"github.com/BwCloudWeGo/bw-cli/internal/user/service"
)

type memoryUserRepo struct {
	nextID    int64
	byID      map[string]*model.User
	byAccount map[string]*model.User
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		nextID:    1,
		byID:      map[string]*model.User{},
		byAccount: map[string]*model.User{},
	}
}

func (r *memoryUserRepo) Save(_ context.Context, user *model.User) error {
	if user.ID == "" {
		user.ID = strconv.FormatInt(r.nextID, 10)
		r.nextID++
	}
	r.byID[user.ID] = user
	r.byAccount[user.Account] = user
	return nil
}

func (r *memoryUserRepo) FindByID(_ context.Context, id string) (*model.User, error) {
	user, ok := r.byID[id]
	if !ok {
		return nil, model.ErrUserNotFound
	}
	return user, nil
}

func (r *memoryUserRepo) FindByAccount(_ context.Context, account string) (*model.User, error) {
	user, ok := r.byAccount[account]
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

func TestLoginReturnsUserOnly(t *testing.T) {
	svc := newTestService(newMemoryUserRepo())
	_, err := svc.Register(context.Background(), dto.RegisterCommand{
		Account:     "grace",
		DisplayName: "Grace",
		Password:    "secret123",
	})
	require.NoError(t, err)

	user, err := svc.Login(context.Background(), dto.LoginCommand{
		Account:  "grace",
		Password: "secret123",
	})

	require.NoError(t, err)
	require.Equal(t, "grace", user.Account)
}

func TestRegisterCreatesUserAndRejectsDuplicateAccount(t *testing.T) {
	svc := newTestService(newMemoryUserRepo())

	created, err := svc.Register(context.Background(), dto.RegisterCommand{
		Account:     "ada",
		DisplayName: "Ada",
		Password:    "secret123",
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "ada", created.Account)
	require.Equal(t, "Ada", created.DisplayName)

	_, err = svc.Register(context.Background(), dto.RegisterCommand{
		Account:     "ada",
		DisplayName: "Ada Again",
		Password:    "secret123",
	})
	require.ErrorIs(t, err, model.ErrAccountAlreadyExists)
}

func TestLoginValidatesPassword(t *testing.T) {
	svc := newTestService(newMemoryUserRepo())
	_, err := svc.Register(context.Background(), dto.RegisterCommand{
		Account:     "grace",
		DisplayName: "Grace",
		Password:    "secret123",
	})
	require.NoError(t, err)

	user, err := svc.Login(context.Background(), dto.LoginCommand{
		Account:  "grace",
		Password: "secret123",
	})
	require.NoError(t, err)
	require.Equal(t, "grace", user.Account)

	_, err = svc.Login(context.Background(), dto.LoginCommand{
		Account:  "grace",
		Password: "wrong",
	})
	require.ErrorIs(t, err, model.ErrInvalidCredentials)
}

func newTestService(repo model.Repository) *service.Service {
	return service.NewService(repo, plainHasher{})
}
