package auth

import (
	"errors"
	"os"
	"path/filepath"
)

// lastLoginPath returns the full path to the file storing last successful login name.
func lastLoginPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "GophKeeper")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(p, "last_login"), nil
}

// SaveLastLogin stores the provided login as the current user context for the CLI.
func SaveLastLogin(login string) error {
	if login == "" {
		return errors.New("empty login")
	}
	p, err := lastLoginPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(login), 0o600)
}

// LoadLastLogin returns last stored login.
func LoadLastLogin() (string, error) {
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
	// Trim simple trailing whitespace
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return string(b), nil
}
