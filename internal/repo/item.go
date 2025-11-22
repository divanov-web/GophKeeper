package repo

import (
	"GophKeeper/internal/model"
	"context"
	"time"

	"gorm.io/gorm"
)

// ItemRepository определяет минимальный контракт доступа к Item для слоя сервиса.
// Реализация пока-заглушка: методы не делают фактических операций.
type ItemRepository interface {
	// GetItemsUpdatedSince возвращает элементы пользователя, изменённые после указанного времени.
	GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error)

	// UpsertItems делает вставку/обновление записей пользователя по их ID.
	UpsertItems(ctx context.Context, userID int64, items []model.Item) error
}

type itemRepo struct {
	db *gorm.DB
}

// NewItemRepository создаёт реализацию репозитория для Item.
func NewItemRepository(db *gorm.DB) ItemRepository {
	return &itemRepo{db: db}
}

func (r *itemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	// Заглушка: вернём пустой список, без обращения к БД
	return []model.Item{}, nil
}

func (r *itemRepo) UpsertItems(ctx context.Context, userID int64, items []model.Item) error {
	// Заглушка: ничего не делаем
	return nil
}
