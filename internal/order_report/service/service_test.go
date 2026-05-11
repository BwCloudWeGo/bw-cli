package service

import (
	"context"
	"testing"

	"github.com/BwCloudWeGo/bw-cli/internal/order_report/dto"
	"github.com/BwCloudWeGo/bw-cli/internal/order_report/model"
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

	created, err := svc.Create(ctx, dto.CreateCommand{CustomerName: "first-customer_name", Status: "first-status", TotalAmount: "first-total_amount"})
	require.NoError(t, err)
	require.NotZero(t, created.CustomerName)

	got, err := svc.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, created.ID, got.ID)

	list, err := svc.List(ctx, dto.ListCommand{Page: 1, PageSize: 10})
	require.NoError(t, err)
	require.Equal(t, int64(1), list.Total)
	require.Len(t, list.Items, 1)

	updated, err := svc.Update(ctx, dto.UpdateCommand{ID: created.ID, CustomerName: "updated-customer_name", Status: "updated-status", TotalAmount: "updated-total_amount"})
	require.NoError(t, err)
	require.NotZero(t, updated.CustomerName)

	require.NoError(t, svc.Delete(ctx, created.ID))
	_, err = svc.Get(ctx, created.ID)
	require.ErrorIs(t, err, model.ErrOrderReportNotFound)
}

type fakeRepository struct {
	items map[int32]*model.OrderReport
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{items: make(map[int32]*model.OrderReport)}
}

func (r *fakeRepository) Save(ctx context.Context, item *model.OrderReport) error {
	copy := *item
	r.items[item.ID] = &copy
	return nil
}

func (r *fakeRepository) FindByID(ctx context.Context, id int32) (*model.OrderReport, error) {
	item, ok := r.items[id]
	if !ok {
		return nil, model.ErrOrderReportNotFound
	}
	copy := *item
	return &copy, nil
}

func (r *fakeRepository) List(ctx context.Context, offset int, limit int) ([]*model.OrderReport, int64, error) {
	items := make([]*model.OrderReport, 0, len(r.items))
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

func (r *fakeRepository) Delete(ctx context.Context, id int32) error {
	if _, ok := r.items[id]; !ok {
		return model.ErrOrderReportNotFound
	}
	delete(r.items, id)
	return nil
}

var _ model.Repository = (*fakeRepository)(nil)
