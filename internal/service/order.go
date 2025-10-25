package service

import (
	"context"
	"errors"
	"github.com/divanov-web/gophermart/internal/accrual"
	"github.com/divanov-web/gophermart/internal/config"
	"github.com/divanov-web/gophermart/internal/model"
	"github.com/divanov-web/gophermart/internal/repository"
	"github.com/divanov-web/gophermart/internal/utils"
	"go.uber.org/zap"
	"gorm.io/gorm"
	"time"
)

var (
	ErrOrderExists          = errors.New("order already uploaded")
	ErrOrderOwnedByOther    = errors.New("order already uploaded by another user")
	ErrInvalidOrderNumber   = errors.New("wrong order number")
	ErrInvalidWithdrawOrder = errors.New("invalid order number format")
	ErrInsufficientFunds    = errors.New("not enough funds")
	ErrNegativeWithdraw     = errors.New("withdraw sum must be positive")
)

type OrderService struct {
	repo     repository.OrderRepository
	userRepo repository.UserRepository
	logger   *zap.SugaredLogger
	config   *config.Config
}

type WithdrawalRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

func NewOrderService(repo repository.OrderRepository, userRepo repository.UserRepository, logger *zap.SugaredLogger, config *config.Config) *OrderService {
	return &OrderService{
		repo:     repo,
		userRepo: userRepo,
		logger:   logger,
		config:   config,
	}
}

// UploadOrder загружает новый заказ
func (s *OrderService) UploadOrder(ctx context.Context, userID int64, number string) error {
	if !utils.IsValidLuhn(number) {
		return ErrInvalidOrderNumber
	}

	order, err := s.repo.GetByNumber(ctx, number)
	if err == nil {
		if order.UserID == userID {
			return ErrOrderExists
		}
		return ErrOrderOwnedByOther
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// если заказ не найден, создаём новый
	newOrder := &model.Order{
		Number: number,
		UserID: userID,
		Status: model.OrderStatusNew,
	}
	return s.repo.Create(ctx, newOrder)
}

func (s *OrderService) GetOrdersByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	return s.repo.GetByUserID(ctx, userID)
}

// StartOrderSenderWorker Создаёт горутину, отправляет заказы в Accrual (только для локального сервера)
func (s *OrderService) StartOrderSenderWorker(ctx context.Context, interval time.Duration, client *accrual.Client) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.processNewOrders(ctx, client)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// processNewOrders Отправляет заказы в Accrual и меняет статус с NEW на PROCESSING
func (s *OrderService) processNewOrders(ctx context.Context, client *accrual.Client) {
	orders, err := s.repo.GetByStatus(ctx, model.OrderStatusNew)
	if err != nil {
		// todo лог ошибки
		return
	}

	for _, order := range orders {
		if s.config.SendOrders {
			err := client.SendOrder(order.Number)
			if err != nil {
				s.logger.Errorw("failed to send order to accrual", "error", err)
				continue
			}
		}

		// Обновляем статус заказа на PROCESSING
		order.Status = model.OrderStatusProcessing
		_ = s.repo.Update(ctx, &order)
	}
}

// StartAccrualUpdaterWorker Создаёт горутину, периодически проверяет статус заказа в Accrual
func (s *OrderService) StartAccrualUpdaterWorker(ctx context.Context, interval time.Duration, client *accrual.Client) {
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.updateProcessingOrders(ctx, client)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// updateProcessingOrders Проверяет заказы в Accrual и меняет статус с PROCESSING на INVALID или PROCESSED
func (s *OrderService) updateProcessingOrders(ctx context.Context, client *accrual.Client) {
	orders, err := s.repo.GetByStatus(ctx, model.OrderStatusProcessing)
	if err != nil {
		// TODO: лог ошибки
		return
	}

	for _, order := range orders {
		resp, err := client.GetOrderInfo(order.Number)
		if err != nil || resp == nil {
			// TODO: лог ошибки или пропуск необработанного заказа
			continue
		}

		switch resp.Status {
		case "REGISTERED", "PROCESSING":
			// оставим без изменений
			continue
		case "PROCESSED":
			order.Status = model.OrderStatusProcessed
			order.Accrual = resp.Accrual
			if err := s.repo.Update(ctx, &order); err != nil {
				// лог ошибки
				continue
			}
			if resp.Accrual != nil {
				err := s.userRepo.IncreaseBalance(ctx, order.UserID, *resp.Accrual)
				if err == nil {
					s.logger.Infow(
						"User balance increased",
						"userID", order.UserID,
						"sum", *resp.Accrual,
					)
				} else {
					s.logger.Errorw(
						"Failed to increase user balance",
						"error", err,
					)
				}
			}
		case "INVALID":
			order.Status = model.OrderStatusInvalid
			_ = s.repo.Update(ctx, &order)
			s.logger.Infow(
				"Order status invalid in accrual",
				"OrderId", order.ID,
			)
		default:
			// TODO: лог неизвестного статуса
			continue
		}
	}
}

func (s *OrderService) Withdraw(ctx context.Context, userID int64, req WithdrawalRequest) error {
	if !utils.IsValidLuhn(req.Order) {
		return ErrInvalidWithdrawOrder
	}

	if req.Sum <= 0 {
		return ErrNegativeWithdraw
	}

	err := s.userRepo.WithdrawBalance(ctx, userID, req.Sum, req.Order)
	if errors.Is(err, repository.ErrLowBalance) {
		return ErrInsufficientFunds
	}

	return err
}
