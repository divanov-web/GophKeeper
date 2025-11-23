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

	// UpsertLogin устанавливает/обновляет зашифрованный логин для записи name.
	// Если записи нет — создаёт её. Возвращает id записи и флаг created=true, если запись была создана.
	UpsertLogin(name string, loginCipher, loginNonce []byte) (id string, created bool, err error)

	// UpsertPassword устанавливает/обновляет зашифрованный пароль для записи name.
	UpsertPassword(name string, passCipher, passNonce []byte) (id string, created bool, err error)

	// UpsertText устанавливает/обновляет зашифрованный произвольный текст для записи name.
	UpsertText(name string, textCipher, textNonce []byte) (id string, created bool, err error)

	// UpsertCard устанавливает/обновляет зашифрованные данные карты (JSON) для записи name.
	UpsertCard(name string, cardCipher, cardNonce []byte) (id string, created bool, err error)

	// UpsertFile сохраняет зашифрованный файл в таблицу blobs и проставляет связь в items.
	UpsertFile(name, fileName string, blobCipher, blobNonce []byte) (id string, created bool, err error)

	// SetServerVersion устанавливает серверную версию для записи по id
	SetServerVersion(id string, version int64) error

	// GetBlobByID возвращает блоб по идентификатору
	GetBlobByID(id string) (*model.Blob, error)

	// UpsertFullFromServer полностью вставляет/обновляет запись items по снимку с сервера
	UpsertFullFromServer(it model.Item) error
}
