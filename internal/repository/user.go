package repository

import (
	"GophKeeper/internal/model"
	"context"
	"errors"
	"gorm.io/gorm"
)

var (
	ErrLowBalance = errors.New("insufficient funds")
)

type UserRepository interface {
	CreateUser(ctx context.Context, user *model.User) (*model.User, error)
	GetUserByLogin(ctx context.Context, login string) (*model.User, error)
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
