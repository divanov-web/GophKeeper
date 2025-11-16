package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
)

// key length for AES-256
const keyLen = 32

// keyFilePath returns path to per-user key file alongside the SQLite DB, under the same base directory logic.
func keyFilePath(login string) (string, error) {
	if login == "" {
		return "", errors.New("empty login for key path")
	}
	base := os.Getenv("CLIENT_DB_PATH")
	if base == "" {
		cfgDir, err := os.UserConfigDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(cfgDir, "GophKeeper", "users")
	}
	dir := filepath.Join(base, login)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "key.bin"), nil
}

// LoadOrCreateKey loads an existing key for the user or creates a new random one.
func LoadOrCreateKey(login string) ([]byte, error) {
	path, err := keyFilePath(login)
	if err != nil {
		return nil, err
	}
	if b, err := os.ReadFile(path); err == nil {
		if len(b) != keyLen {
			return nil, errors.New("invalid key length")
		}
		return b, nil
	}
	// create new
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	// write with restricted perms
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt encrypts plain using AES-GCM with the provided key. Returns cipher and nonce.
func Encrypt(plain []byte, key []byte) ([]byte, []byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	out := gcm.Seal(nil, nonce, plain, nil)
	return out, nonce, nil
}

// Decrypt decrypts cipher using AES-GCM with key and nonce.
func Decrypt(ciphertext, nonce, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}
