package service

import (
	"GophKeeper/internal/cli/model"
	view "GophKeeper/internal/cli/model/view"
)

// ItemService описывает юзкейс-уровень работы с локальными записями (items) для CLI.
type ItemService interface {
	// Add создаёт новую запись и, при наличии, сразу сохраняет логин/пароль (могут быть nil). Возвращает ID.
	Add(name string, login, password *string) (string, error)

	// List возвращает список всех записей текущего пользователя.
	List() ([]model.Item, error)

	// GetByName возвращает запись по точному имени (DTO с расшифрованными полями).
	GetByName(name string) (*view.DecryptedItem, error)

	// Edit устанавливает/обновляет одно из полей: login|password|text|card|file.
	// Для типов:
	// - login/password/text: value содержит ровно один элемент — строку
	// - card: value содержит 4 элемента: number, card_holder, exp, cvc (будут упакованы в JSON и зашифрованы)
	// - file: value содержит 1 элемент — путь к файлу, который будет прочитан, зашифрован и сохранён во внутреннее хранилище
	// Возвращает id записи и признак created=true, если запись была создана.
	Edit(name, fieldType string, value []string) (id string, created bool, err error)
}
