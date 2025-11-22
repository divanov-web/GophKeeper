package service

import (
	"GophKeeper/internal/model"
	"GophKeeper/internal/repo"
	"context"
	"time"
)

// ItemService инкапсулирует бизнес-логику работы с Item.
// Пока реализован как заглушка для будущей синхронизации.
type ItemService struct {
	repo repo.ItemRepository
}

func NewItemService(r repo.ItemRepository) *ItemService {
	return &ItemService{repo: r}
}

// SyncChange описывает минимальную модель изменения элемента для сервиса.
// В заглушке не используется, но оставлена для будущей реализации.
type SyncChange struct {
	ID      string
	Version *int64
	Deleted *bool
	// Остальные поля будут добавлены по мере реализации
}

// SyncRequest вход сервиса синхронизации.
type SyncRequest struct {
	LastSyncAt *time.Time
	Changes    []SyncChange
}

// SyncResult результат синхронизации.
type SyncResult struct {
	Applied       []any
	Conflicts     []any
	ServerChanges []model.Item
	ServerTime    time.Time
}

// Sync — заглушка: ничего не делает и возвращает пустой результат с текущим временем сервера.
func (s *ItemService) Sync(ctx context.Context, userID int64, req SyncRequest) (SyncResult, error) {
	// В будущем здесь будет транзакция, upsert изменений и выборка изменений с сервера.
	return SyncResult{
		Applied:       []any{},
		Conflicts:     []any{},
		ServerChanges: []model.Item{},
		ServerTime:    time.Now().UTC(),
	}, nil
}
