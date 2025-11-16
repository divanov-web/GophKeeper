package service

import (
	"GophKeeper/internal/cli/model"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
	"fmt"
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

// ItemServiceLocal — локальная реализация ItemService поверх SQLite-репозитория.
type ItemServiceLocal struct{}

// ensure interface impl at compile-time
var _ ItemService = (*ItemServiceLocal)(nil)

// withRepo открывает БД текущего пользователя и передаёт инициализированный репозиторий в коллбек.
func (ItemServiceLocal) withRepo(fn func(r *reposqlite.ItemRepositorySQLite) error) error {
	login, err := (fsrepo.AuthFSStore{}).LoadLogin()
	if err != nil {
		return fmt.Errorf("нет активного пользователя: выполните login/register: %w", err)
	}
	r, _, err := reposqlite.OpenForUser(login)
	if err != nil {
		return fmt.Errorf("open user db: %w", err)
	}
	defer r.Close()
	if err := r.Migrate(); err != nil {
		return fmt.Errorf("migrate user db: %w", err)
	}
	return fn(r)
}

// Add item to DB.
func (s ItemServiceLocal) Add(name string) (string, error) {
	var id string
	err := s.withRepo(func(r *reposqlite.ItemRepositorySQLite) error {
		newID, err := r.AddItem(name)
		if err != nil {
			return err
		}
		id = newID
		return nil
	})
	if err != nil {
		return "", err
	}
	return id, nil
}

// List items.
func (s ItemServiceLocal) List() ([]model.Item, error) {
	var out []model.Item
	err := s.withRepo(func(r *reposqlite.ItemRepositorySQLite) error {
		list, err := r.ListItems()
		if err != nil {
			return err
		}
		out = list
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetByName item by name.
func (s ItemServiceLocal) GetByName(name string) (*model.Item, error) {
	var item *model.Item
	err := s.withRepo(func(r *reposqlite.ItemRepositorySQLite) error {
		it, err := r.GetItemByName(name)
		if err != nil {
			return err
		}
		item = it
		return nil
	})
	if err != nil {
		return nil, err
	}
	return item, nil
}
