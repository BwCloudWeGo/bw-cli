package service

import (
	"context"
	"testing"

	"github.com/BwCloudWeGo/bw-cli/internal/order/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewService(t *testing.T) {
	svc := NewService(nil, zap.NewNop())

	require.NotNil(t, svc)
}

func TestServiceCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newFakeRepository()
	svc := NewService(repo, zap.NewNop())

	created, err := svc.Create(ctx, dto.CreateCommand{Name: "first", Description: "created from service test"})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)
	require.Equal(t, "first", created.Name)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	list, err := svc.List(ctx, dto.ListCommand{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Len(t, list.Items, 1)

	updated, err := svc.Update(ctx, dto.UpdateCommand{ID: created.ID, Name: "updated", Description: "updated from service test"})
	require.NoError(t, err)
	require.Equal(t, "updated", updated.Name)

	require.NoError(t, svc.Delete(ctx, created.ID))
	_, err = svc.Get(ctx, created.ID)
	require.ErrorIs(t, err, model.ErrOrderNotFound)
}

type fakeRepository struct {
	items map[string]*model.Order
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[string]*model.Order)}
}

func (r *fakeRepository) Save(ctx context.Context, item *model.Order) error {
	copy := *item
	r.items[item.ID] = &copy
	return nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id string) (*model.Order, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, model.ErrOrderNotFound
	}
	copy := *item
	return &copy, nil
}

func (r *fakeRepository) List(ctx context.Context, offset int, limit int) ([]*model.Order, int64, error) {
	items := make([]*model.Order, 0, len(r.items))
	for _, item := range r.items {
		copy := *item
		items = append(items, &copy)
	}
	if offset > len(items) {
		return nil, int64(len(items)), nil
	}
	end := offset + limit
	if end > len(items) {
		end = len(items)
	}
	return items[offset:end], int64(len(items)), nil
}

func (r *fakeRepository) Delete(ctx context.Context, id string) error {
	if _, ok := r.items[id]; !ok {
		return model.ErrOrderNotFound
	}
	delete(r.items, id)
	return nil
}

var _ model.Repository = (*fakeRepository)(nil)
