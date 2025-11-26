package repo

import (
	"GophKeeper/internal/model"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestUserRepository_CreateAndGet(t *testing.T) {
	db := newTestDB(t)
	r := NewUserRepository(db)
	ctx := context.Background()

	// успешное создание
	u, err := r.CreateUser(ctx, &model.User{Login: "john", Password: "hash"})
	assert.NoError(t, err)
	assert.NotZero(t, u.ID)

	// поиск по логину — найдено
	got, err := r.GetUserByLogin(ctx, "john")
	assert.NoError(t, err)
	assert.Equal(t, u.ID, got.ID)

	// уникальный логин — вторая вставка должна дать ошибку
	_, err = r.CreateUser(ctx, &model.User{Login: "john", Password: "x"})
	assert.Error(t, err)

	// поиск несуществующего — ожидаем gorm.ErrRecordNotFound
	got, err = r.GetUserByLogin(ctx, "doesnotexist")
	assert.Nil(t, got)
	assert.Error(t, err)
	assert.Equal(t, gorm.ErrRecordNotFound, err)
}
