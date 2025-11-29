package service

import (
	"GophKeeper/internal/model"
	"GophKeeper/internal/repo"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/crypto/bcrypt"
)

// мок для repo.UserRepository
type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	args := m.Called(ctx, user)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.UserRepository = (*mockUserRepo)(nil)

func TestUserService_Register(t *testing.T) {
	ctx := context.Background()
	m := new(mockUserRepo)
	svc := NewUserService(m)

	t.Run("ok when login free", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "john").Return((*model.User)(nil), nil).Once()
		created := &model.User{ID: 10, Login: "john"}
		m.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool {
			return u.Login == "john" && u.Password != ""
		})).Return(created, nil).Once()

		user, err := svc.Register(ctx, "john", "p@ss")
		assert.NoError(t, err)
		assert.Equal(t, int64(10), user.ID)
		m.AssertExpectations(t)
	})

	t.Run("conflict when login taken", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "john").Return(&model.User{ID: 1, Login: "john"}, nil).Once()

		user, err := svc.Register(ctx, "john", "p@ss")
		assert.Nil(t, user)
		assert.ErrorIs(t, err, ErrLoginTaken)
		m.AssertExpectations(t)
	})
}

func TestUserService_Login(t *testing.T) {
	ctx := context.Background()
	m := new(mockUserRepo)
	svc := NewUserService(m)

	// готовим хеш для пароля "secret"
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)

	t.Run("ok with valid credentials", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "alice").Return(&model.User{ID: 2, Login: "alice", Password: string(hash)}, nil).Once()

		user, err := svc.Login(ctx, "alice", "secret")
		assert.NoError(t, err)
		assert.Equal(t, int64(2), user.ID)
		m.AssertExpectations(t)
	})

	t.Run("invalid password", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "alice").Return(&model.User{ID: 2, Login: "alice", Password: string(hash)}, nil).Once()

		user, err := svc.Login(ctx, "alice", "wrong")
		assert.Nil(t, user)
		assert.Error(t, err)
		m.AssertExpectations(t)
	})
}
