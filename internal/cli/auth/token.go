package auth

import (
	"errors"
	"os"
	"path/filepath"
)

// AuthTokenPath returns the full path to the auth token file under the user's config directory.
func AuthTokenPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, "GophKeeper")
	if err := os.MkdirAll(p, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(p, "auth_token"), nil
}

// SaveToken writes token to the auth token file.
func SaveToken(token string) error {
	p, err := AuthTokenPath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte(token), 0o600)
}

// LoadToken reads token from the auth token file.
func LoadToken() (string, error) {
	p, err := AuthTokenPath()
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
	// Trim any trailing newlines/spaces
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return string(b), nil
}
