package mocks

import (
	"context"

	"github.com/divanov-web/gophermart/internal/model"
	"github.com/stretchr/testify/mock"
)

type MockOrderRepo struct {
	mock.Mock
}

func (m *MockOrderRepo) Create(ctx context.Context, order *model.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrderRepo) GetByNumber(ctx context.Context, number string) (*model.Order, error) {
	args := m.Called(ctx, number)
	o := args.Get(0)
	if o == nil {
		return nil, args.Error(1)
	}
	return o.(*model.Order), args.Error(1)
}

func (m *MockOrderRepo) GetByUserID(ctx context.Context, userID int64) ([]model.Order, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]model.Order), args.Error(1)
}

func (m *MockOrderRepo) GetByStatus(ctx context.Context, status model.OrderStatus) ([]model.Order, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]model.Order), args.Error(1)
}

func (m *MockOrderRepo) Update(ctx context.Context, order *model.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}
