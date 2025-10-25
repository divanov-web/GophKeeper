package repository

import (
	"context"
	"errors"
	"github.com/divanov-web/gophermart/internal/model"
	"gorm.io/gorm"
)

var (
	ErrLowBalance = errors.New("insufficient funds")
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) (*model.User, error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
	IncreaseBalance(ctx context.Context, userID int64, amount float64) error
	WithdrawBalance(ctx context.Context, userID int64, amount float64, order string) error
	GetWithdrawalsByUser(ctx context.Context, userID int64) ([]model.Withdrawal, error)
	GetBalance(ctx context.Context, userID int64) (float64, float64, error)
}

type userRepo struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepo{db: db}
}

func (r *userRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	err := r.db.WithContext(ctx).Create(user).Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *userRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	var user model.User
	if err := r.db.WithContext(ctx).Where("login = ?", login).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *userRepo) IncreaseBalance(ctx context.Context, userID int64, amount float64) error {
	return r.db.WithContext(ctx).
		Model(&model.User{}).
		Where("id = ?", userID).
		Update("balance", gorm.Expr("balance + ?", amount)).
		Error
}

func (r *userRepo) WithdrawBalance(ctx context.Context, userID int64, amount float64, order string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.First(&user, userID).Error; err != nil {
			return err
		}

		if user.Balance < amount {
			return ErrLowBalance
		}

		if err := tx.Model(&model.User{}).
			Where("id = ?", userID).
			Updates(map[string]interface{}{
				"balance":   gorm.Expr("balance - ?", amount),
				"withdrawn": gorm.Expr("withdrawn + ?", amount),
			}).Error; err != nil {
			return err
		}

		withdrawal := &model.Withdrawal{
			UserID: userID,
			Order:  order,
			Sum:    amount,
		}

		if err := tx.Create(withdrawal).Error; err != nil {
			return err
		}

		return nil
	})
}

func (r *userRepo) GetWithdrawalsByUser(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	var withdrawals []model.Withdrawal
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("processed_at desc").
		Find(&withdrawals).Error
	return withdrawals, err
}

func (r *userRepo) GetBalance(ctx context.Context, userID int64) (float64, float64, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Select("balance", "withdrawn").
		Where("id = ?", userID).
		First(&user).Error
	if err != nil {
		return 0, 0, err
	}
	return user.Balance, user.Withdrawn, nil
}
