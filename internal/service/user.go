package service

import (
	"context"
	"errors"

	"github.com/divanov-web/gophermart/internal/model"
	"github.com/divanov-web/gophermart/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type UserService struct {
	repo repository.UserRepository
}

type BalanceResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

var ErrLoginTaken = errors.New("login already in use")

func NewUserService(repo repository.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Register регистрирует нового пользователя
func (s *UserService) Register(ctx context.Context, login, password string) (*model.User, error) {
	existing, _ := s.repo.GetUserByLogin(ctx, login)
	if existing != nil {
		return nil, ErrLoginTaken
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Login:    login,
		Password: string(hashed),
	}

	return s.repo.CreateUser(ctx, user)
}

// Login проверяет логин/пароль и возвращает пользователя
func (s *UserService) Login(ctx context.Context, login, password string) (*model.User, error) {
	user, err := s.repo.GetUserByLogin(ctx, login)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}

	return user, nil
}

func (s *UserService) GetUserBalance(ctx context.Context, userID int64) (*BalanceResponse, error) {
	current, withdrawn, err := s.repo.GetBalance(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &BalanceResponse{
		Current:   current,
		Withdrawn: withdrawn,
	}, nil
}

func (s *UserService) GetWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	return s.repo.GetWithdrawalsByUser(ctx, userID)
}
