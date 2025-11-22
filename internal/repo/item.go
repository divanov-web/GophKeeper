package repo

import (
	"GophKeeper/internal/model"
	"context"
	"time"

	"gorm.io/gorm"
)

// ItemRepository определяет минимальный контракт доступа к Item для слоя сервиса.
type ItemRepository interface {
	// GetItemsUpdatedSince возвращает элементы пользователя, изменённые после указанного времени.
	GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error)

	// GetByID возвращает элемент по id и userID.
	GetByID(ctx context.Context, userID int64, id string) (*model.Item, error)

	// Create вставляет новую запись.
	Create(ctx context.Context, it *model.Item) error

	// UpdateWithVersion выполняет обновление c проверкой версии (OCC):
	// WHERE id=? AND user_id=? AND version=?; увеличивает версию на 1 и возвращает новое значение версии.
	UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error)
}

type itemRepo struct {
	db *gorm.DB
}

// NewItemRepository создаёт реализацию репозитория для Item.
func NewItemRepository(db *gorm.DB) ItemRepository {
	return &itemRepo{db: db}
}

// GetItemsUpdatedSince возвращает элементы, обновлённые после времени since.
func (r *itemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	var items []model.Item
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND updated_at > ?", userID, since).
		Order("updated_at asc").
		Find(&items).Error
	if err != nil {
		return nil, err
	}
	return items, nil
}

// GetByID возвращает элемент по идентификатору и пользователю.
func (r *itemRepo) GetByID(ctx context.Context, userID int64, id string) (*model.Item, error) {
	var it model.Item
	err := r.db.WithContext(ctx).Where("id = ? AND user_id = ?", id, userID).First(&it).Error
	if err != nil {
		return nil, err
	}
	return &it, nil
}

// Create создаёт новую запись Item.
func (r *itemRepo) Create(ctx context.Context, it *model.Item) error {
	return r.db.WithContext(ctx).Create(it).Error
}

// UpdateWithVersion обновляет запись с проверкой версии.
func (r *itemRepo) UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error) {
	// Принудительно выставим updated_at и инкрементируем версию
	now := time.Now().UTC()
	if updates == nil {
		updates = map[string]any{}
	}
	updates["updated_at"] = now

	newVersion := expectedVersion + 1
	updates["version"] = newVersion

	tx := r.db.WithContext(ctx).Model(&model.Item{}).
		Where("id = ? AND user_id = ? AND version = ?", id, userID, expectedVersion).
		Updates(updates)

	if tx.Error != nil {
		return 0, tx.Error
	}
	if tx.RowsAffected == 0 {
		// версия не совпала или записи нет
		return 0, gorm.ErrRecordNotFound
	}
	return newVersion, nil
}
