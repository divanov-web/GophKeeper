package repo

import "GophKeeper/internal/cli/model"

// ItemRepository определяет порт доступа к локальному хранилищу элементов.
type ItemRepository interface {
	// AddItem добавляет запись и возвращает её ID.
	AddItem(name string) (string, error)

	// ListItems возвращает все записи
	ListItems() ([]model.Item, error)

	// GetItemByName находит запись по точному имени.
	GetItemByName(name string) (*model.Item, error)
}
