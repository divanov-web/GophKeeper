package repo

import "GophKeeper/internal/cli/model"

// ItemRepository определяет порт доступа к локальному хранилищу элементов.
type ItemRepository interface {
	// AddEncrypted добавляет запись, принимая уже зашифрованные значения (или nil).
	// Возвращает ID созданной записи.
	AddEncrypted(name string, loginCipher, loginNonce, passCipher, passNonce []byte) (string, error)

	// ListItems возвращает все записи
	ListItems() ([]model.Item, error)

	// GetItemByName находит запись по точному имени.
	GetItemByName(name string) (*model.Item, error)
}
