package service

import (
	"GophKeeper/internal/cli/crypto"
	"GophKeeper/internal/cli/model"
	view "GophKeeper/internal/cli/model/view"
	"GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"fmt"
	"os"
	"path/filepath"
)

// ItemServiceLocal — локальная реализация ItemService.
type ItemServiceLocal struct {
	repo repo.ItemRepository
}

// NewItemServiceLocal создаёт новый сервис работы с items поверх переданного репозитория.
func NewItemServiceLocal(r repo.ItemRepository) ItemService {
	return &ItemServiceLocal{repo: r}
}

// Add создаёт запись: шифрует переданные поля (если заданы) и сохраняет через репозиторий.
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

// List возвращает список всех элементов пользователя.
func (s ItemServiceLocal) List() ([]model.Item, error) {
	return s.repo.ListItems()
}

// GetByName возвращает DTO с расшифрованными полями по точному имени.
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
	// Определяем, что необходимо расшифровать
	needLogin := len(it.LoginCipher) > 0 && len(it.LoginNonce) > 0
	needPass := len(it.PasswordCipher) > 0 && len(it.PasswordNonce) > 0
	needText := len(it.TextCipher) > 0 && len(it.TextNonce) > 0
	needCard := len(it.CardCipher) > 0 && len(it.CardNonce) > 0

	// Значения по умолчанию, если поле отсутствует
	if !needLogin {
		dto.Login = "<not set>"
	}
	if !needPass {
		dto.Password = "<not set>"
	}
	if !needText {
		dto.Text = "<not set>"
	}
	if !needCard {
		dto.Card = "<not set>"
	}
	if it.FileName == "" {
		dto.FileName = "<not set>"
	} else {
		dto.FileName = it.FileName
	}

	// Если нечего расшифровывать — возвращаем уже подготовленный DTO
	if !(needLogin || needPass || needText || needCard) {
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
		if needText {
			dto.Text = "<decrypt error>"
		}
		if needCard {
			dto.Card = "<decrypt error>"
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
	if needText {
		if plain, err := crypto.Decrypt(it.TextCipher, it.TextNonce, key); err != nil {
			dto.Text = "<decrypt error>"
		} else {
			dto.Text = string(plain)
		}
	}
	if needCard {
		if plain, err := crypto.Decrypt(it.CardCipher, it.CardNonce, key); err != nil {
			dto.Card = "<decrypt error>"
		} else {
			// Хранимое значение — JSON; выводим как есть
			dto.Card = string(plain)
		}
	}
	return dto, nil
}

// Edit обновляет запись: шифрует значение и передаёт в репозиторий.
func (s ItemServiceLocal) Edit(name, fieldType string, value []string) (string, bool, error) {
	// Сначала получаем ключ шифрования текущего пользователя
	var key []byte
	loginName, err := (fsrepo.AuthFSStore{}).LoadLogin()
	if err != nil {
		return "", false, fmt.Errorf("нет активного пользователя: выполните login/register: %w", err)
	}
	key, err = crypto.LoadOrCreateKey(loginName)
	if err != nil {
		return "", false, err
	}
	switch fieldType {
	case "login":
		if len(value) != 1 {
			return "", false, fmt.Errorf("ожидается 1 аргумент для login")
		}
		c, n, err := crypto.Encrypt([]byte(value[0]), key)
		if err != nil {
			return "", false, err
		}
		return s.repo.UpsertLogin(name, c, n)
	case "password":
		if len(value) != 1 {
			return "", false, fmt.Errorf("ожидается 1 аргумент для password")
		}
		c, n, err := crypto.Encrypt([]byte(value[0]), key)
		if err != nil {
			return "", false, err
		}
		return s.repo.UpsertPassword(name, c, n)
	case "text":
		if len(value) != 1 {
			return "", false, fmt.Errorf("ожидается 1 аргумент для text")
		}
		c, n, err := crypto.Encrypt([]byte(value[0]), key)
		if err != nil {
			return "", false, err
		}
		return s.repo.UpsertText(name, c, n)
	case "card":
		if len(value) != 4 {
			return "", false, fmt.Errorf("ожидается 4 аргумента для card: <number> <card_holder> <exp> <cvc>")
		}
		// Упакуем в JSON
		payload := fmt.Sprintf(`{"number":%q,"card_holder":%q,"exp":%q,"cvc":%q}`, value[0], value[1], value[2], value[3])
		c, n, err := crypto.Encrypt([]byte(payload), key)
		if err != nil {
			return "", false, err
		}
		return s.repo.UpsertCard(name, c, n)
	case "file":
		if len(value) != 1 {
			return "", false, fmt.Errorf("ожидается 1 аргумент для file: путь к файлу")
		}
		path := value[0]
		data, err := os.ReadFile(path)
		if err != nil {
			return "", false, fmt.Errorf("чтение файла: %w", err)
		}
		c, _, err := crypto.Encrypt(data, key)
		if err != nil {
			return "", false, err
		}
		fileName := filepath.Base(path)
		// передаём содержимое и имя файла.
		return s.repo.UpsertFile(name, fileName, "", c)
	default:
		return "", false, fmt.Errorf("неизвестный тип: %s (ожидается: login|password|text|card|file)", fieldType)
	}
}
