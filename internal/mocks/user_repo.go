package mocks

import (
	"context"
	"github.com/divanov-web/gophermart/internal/model"
	"github.com/stretchr/testify/mock"
)

type MockUserRepo struct {
	mock.Mock
}

func (m *MockUserRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	args := m.Called(ctx, user)
	return args.Get(0).(*model.User), args.Error(1)
}

func (m *MockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	u := args.Get(0)
	if u == nil {
		return nil, args.Error(1)
	}
	return u.(*model.User), args.Error(1)
}

func (m *MockUserRepo) IncreaseBalance(ctx context.Context, userID int64, amount float64) error {
	return nil
}

func (m *MockUserRepo) WithdrawBalance(ctx context.Context, userID int64, amount float64, order string) error {
	return nil
}

func (m *MockUserRepo) GetBalance(ctx context.Context, userID int64) (float64, float64, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).(float64), args.Get(1).(float64), args.Error(2)
}

func (m *MockUserRepo) GetWithdrawalsByUser(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return nil, nil
}
