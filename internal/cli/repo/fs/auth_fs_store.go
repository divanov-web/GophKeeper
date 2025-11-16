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

// Save token to file.
func (AuthFSStore) Save(token string) error {
	p, err := tokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(token), 0o600)
}

// Load token from a file.
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
	// trim trailing newline/space
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

// SaveLogin to a file.
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

// LoadLogin from a file.
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
	// trim trailing newline/space
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
