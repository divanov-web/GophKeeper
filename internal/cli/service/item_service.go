package service

import (
	"GophKeeper/internal/cli/crypto"
	"GophKeeper/internal/cli/model"
	view "GophKeeper/internal/cli/model/view"
	"GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"fmt"
)

// ItemService описывает юзкейс-уровень работы с локальными записями (items) для CLI.
type ItemService interface {
	// Add создаёт новую запись и, при наличии, сразу сохраняет логин/пароль (могут быть nil). Возвращает ID.
	Add(name string, login, password *string) (string, error)

	// List возвращает список всех записей текущего пользователя.
	List() ([]model.Item, error)

	// GetByName возвращает запись по точному имени (DTO с расшифрованными полями).
	GetByName(name string) (*view.DecryptedItem, error)
}

// ItemServiceLocal - локальная реализация ItemService.
type ItemServiceLocal struct {
	repo repo.ItemRepository
}

// NewItemServiceLocal конструктор сервиса item
func NewItemServiceLocal(r repo.ItemRepository) ItemService {
	return &ItemServiceLocal{repo: r}
}

// Add item: выполняет шифрование (если заданы поля) и передаёт в репозиторий.
func (s ItemServiceLocal) Add(name string, login, password *string) (string, error) {
	var loginCipher, loginNonce, passCipher, passNonce []byte
	if login != nil || password != nil {
		loginName, err := (fsrepo.AuthFSStore{}).LoadLogin()
		if err != nil {
			return "", fmt.Errorf("нет активного пользователя: выполните login/register: %w", err)
		}
		key, err := crypto.LoadOrCreateKey(loginName)
		if err != nil {
			return "", err
		}
		if login != nil {
			c, n, err := crypto.Encrypt([]byte(*login), key)
			if err != nil {
				return "", err
			}
			loginCipher, loginNonce = c, n
		}
		if password != nil {
			c, n, err := crypto.Encrypt([]byte(*password), key)
			if err != nil {
				return "", err
			}
			passCipher, passNonce = c, n
		}
	}
	return s.repo.AddEncrypted(name, loginCipher, loginNonce, passCipher, passNonce)
}

// List items.
func (s ItemServiceLocal) List() ([]model.Item, error) {
	return s.repo.ListItems()
}

// GetByName: читает сырой объект и выполняет расшифровку полей.
func (s ItemServiceLocal) GetByName(name string) (*view.DecryptedItem, error) {
	it, err := s.repo.GetItemByName(name)
	if err != nil {
		return nil, err
	}
	dto := &view.DecryptedItem{
		ID:        it.ID,
		Name:      it.Name,
		CreatedAt: it.CreatedAt,
		UpdatedAt: it.UpdatedAt,
		Version:   it.Version,
		Deleted:   it.Deleted,
	}
	needLogin := len(it.LoginCipher) > 0 && len(it.LoginNonce) > 0
	needPass := len(it.PasswordCipher) > 0 && len(it.PasswordNonce) > 0
	// Значения по умолчанию, если поле отсутствует
	if !needLogin {
		dto.Login = "<not set>"
	}
	if !needPass {
		dto.Password = "<not set>"
	}
	if !(needLogin || needPass) {
		return dto, nil
	}
	loginName, _ := (fsrepo.AuthFSStore{}).LoadLogin()
	key, kerr := crypto.LoadOrCreateKey(loginName)
	if kerr != nil {
		if needLogin {
			dto.Login = "<decrypt error>"
		}
		if needPass {
			dto.Password = "<decrypt error>"
		}
		return dto, nil
	}
	if needLogin {
		if plain, err := crypto.Decrypt(it.LoginCipher, it.LoginNonce, key); err != nil {
			dto.Login = "<decrypt error>"
		} else {
			dto.Login = string(plain)
		}
	}
	if needPass {
		if plain, err := crypto.Decrypt(it.PasswordCipher, it.PasswordNonce, key); err != nil {
			dto.Password = "<decrypt error>"
		} else {
			dto.Password = string(plain)
		}
	}
	return dto, nil
}
