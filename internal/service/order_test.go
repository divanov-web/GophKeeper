package service_test

import (
	"context"
	"testing"

	"github.com/divanov-web/gophermart/internal/mocks"
	"github.com/divanov-web/gophermart/internal/model"
	"github.com/divanov-web/gophermart/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"gorm.io/gorm"
)

func TestUploadOrder_NewOrder(t *testing.T) {
	orderRepo := new(mocks.MockOrderRepo)
	userRepo := new(mocks.MockUserRepo)
	svc := service.NewOrderService(orderRepo, userRepo, nil, nil)

	ctx := context.Background()
	orderNumber := "79927398713" // валидный номер по Луну
	userID := int64(1)

	orderRepo.On("GetByNumber", ctx, orderNumber).Return(nil, gorm.ErrRecordNotFound)
	orderRepo.On("Create", ctx, mock.AnythingOfType("*model.Order")).Return(nil)

	err := svc.UploadOrder(ctx, userID, orderNumber)
	assert.NoError(t, err)
}

func TestUploadOrder_AlreadyOwnedByUser(t *testing.T) {
	orderRepo := new(mocks.MockOrderRepo)
	userRepo := new(mocks.MockUserRepo)
	svc := service.NewOrderService(orderRepo, userRepo, nil, nil)

	ctx := context.Background()
	orderNumber := "79927398713"
	userID := int64(1)

	order := &model.Order{Number: orderNumber, UserID: userID}
	orderRepo.On("GetByNumber", ctx, orderNumber).Return(order, nil)

	err := svc.UploadOrder(ctx, userID, orderNumber)
	assert.ErrorIs(t, err, service.ErrOrderExists)
}

func TestUploadOrder_OwnedByOtherUser(t *testing.T) {
	orderRepo := new(mocks.MockOrderRepo)
	userRepo := new(mocks.MockUserRepo)
	svc := service.NewOrderService(orderRepo, userRepo, nil, nil)

	ctx := context.Background()
	orderNumber := "79927398713"
	userID := int64(1)
	otherUserID := int64(2)

	order := &model.Order{Number: orderNumber, UserID: otherUserID}
	orderRepo.On("GetByNumber", ctx, orderNumber).Return(order, nil)

	err := svc.UploadOrder(ctx, userID, orderNumber)
	assert.ErrorIs(t, err, service.ErrOrderOwnedByOther)
}

func TestUploadOrder_InvalidNumber(t *testing.T) {
	orderRepo := new(mocks.MockOrderRepo)
	userRepo := new(mocks.MockUserRepo)
	svc := service.NewOrderService(orderRepo, userRepo, nil, nil)

	ctx := context.Background()
	orderNumber := "12345" // невалидный номер
	userID := int64(1)

	err := svc.UploadOrder(ctx, userID, orderNumber)
	assert.ErrorIs(t, err, service.ErrInvalidOrderNumber)
}
