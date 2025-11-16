package repo

import "GophKeeper/internal/cli/model"

// ItemRepository определяет порт доступа к локальному хранилищу элементов.
type ItemRepository interface {
	// Add добавляет запись и, при наличии, сразу сохраняет логин/пароль (могут быть nil).
	// Возвращает ID созданной записи.
	Add(name string, login, password *string) (string, error)

	// ListItems возвращает все записи
	ListItems() ([]model.Item, error)

	// GetItemByName находит запись по точному имени.
	GetItemByName(name string) (*model.Item, error)
}
