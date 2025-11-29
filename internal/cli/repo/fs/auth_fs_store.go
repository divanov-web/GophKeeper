package fs

import (
	"errors"
	"os"
	"path/filepath"
)

// AuthFSStore — файловое хранилище токена и контекста пользователя для CLI.
type AuthFSStore struct{}

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "GophKeeper")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return p, nil
}

func tokenPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "auth_token"), nil
}

func lastLoginPath() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "last_login"), nil
}

func lastSyncAtPath(login string) (string, error) {
	if login == "" {
		return "", errors.New("empty login for last_sync_at")
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	// Храним per-user, чтобы поддерживать несколько аккаунтов
	safe := login
	return filepath.Join(dir, "last_sync_at_"+safe), nil
}

// Save сохраняет auth‑токен в файл.
func (AuthFSStore) Save(token string) error {
	p, err := tokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(token), 0o600)
}

// Load читает auth‑токен из файла.
func (AuthFSStore) Load() (string, error) {
	p, err := tokenPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty token file")
	}
	// обрезаем завершающие переводы строки/пробелы
	for len(b) > 0 {
		c := b[len(b)-1]
		if c == '\n' || c == '\r' || c == ' ' || c == '\t' {
			b = b[:len(b)-1]
			continue
		}
		break
	}
	return string(b), nil
}

// SaveLogin сохраняет логин пользователя в файл.
func (AuthFSStore) SaveLogin(login string) error {
	if login == "" {
		return errors.New("empty login")
	}
	p, err := lastLoginPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(login), 0o600)
}

// LoadLogin читает логин пользователя из файла.
func (AuthFSStore) LoadLogin() (string, error) {
	p, err := lastLoginPath()
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("no stored login")
	}
	// обрезаем завершающие переводы строки/пробелы
	for len(b) > 0 {
		c := b[len(b)-1]
		if c == '\n' || c == '\r' || c == ' ' || c == '\t' {
			b = b[:len(b)-1]
			continue
		}
		break
	}
	return string(b), nil
}

// SaveLastSyncAt сохраняет значение last_sync_at (RFC3339) для указанного пользователя
func SaveLastSyncAt(login, ts string) error {
	if login == "" {
		return errors.New("empty login")
	}
	p, err := lastSyncAtPath(login)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(ts), 0o600)
}

// LoadLastSyncAt читает last_sync_at для указанного пользователя
func LoadLastSyncAt(login string) (string, error) {
	if login == "" {
		return "", errors.New("empty login")
	}
	p, err := lastSyncAtPath(login)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return "", errors.New("empty last_sync_at file")
	}
	// trim trailing whitespace
	for len(b) > 0 {
		c := b[len(b)-1]
		if c == '\n' || c == '\r' || c == ' ' || c == '\t' {
			b = b[:len(b)-1]
			continue
		}
		break
	}
	return string(b), nil
}
