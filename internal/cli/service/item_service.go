package service

import (
	"GophKeeper/internal/cli/model"
	"GophKeeper/internal/cli/repo"
)

// ItemService описывает юзкейс-уровень работы с локальными записями (items) для CLI.
type ItemService interface {
	// Add создаёт новую запись с указанным именем и возвращает её ID.
	Add(name string) (string, error)

	// List возвращает список всех записей текущего пользователя.
	List() ([]model.Item, error)

	// GetByName возвращает запись по точному имени.
	GetByName(name string) (*model.Item, error)
}

// ItemServiceLocal — локальная реализация ItemService поверх переданного репозитория.
type ItemServiceLocal struct {
	repo repo.ItemRepository
}

// NewItemServiceLocal конструктор сервиса item
func NewItemServiceLocal(r repo.ItemRepository) ItemService {
	return &ItemServiceLocal{repo: r}
}

// Add item to DB.
func (s ItemServiceLocal) Add(name string) (string, error) {
	return s.repo.AddItem(name)
}

// List items.
func (s ItemServiceLocal) List() ([]model.Item, error) {
	return s.repo.ListItems()
}

// GetByName item by name.
func (s ItemServiceLocal) GetByName(name string) (*model.Item, error) {
	return s.repo.GetItemByName(name)
}
