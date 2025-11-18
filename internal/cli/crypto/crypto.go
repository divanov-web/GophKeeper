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

// keyLen — длина ключа для AES‑256 (в байтах).
const keyLen = 32

// keyFilePath возвращает путь к пользовательскому файлу ключа рядом с БД SQLite
// (используется та же логика базового каталога).
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

// LoadOrCreateKey загружает существующий ключ пользователя или создаёт новый случайный.
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
	// создаём новый ключ
	key := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	// записываем с ограниченными правами доступа
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return nil, err
	}
	return key, nil
}

// Encrypt шифрует данные plain с помощью AES‑GCM и заданного ключа.
// Возвращает шифртекст и nonce.
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

// Decrypt расшифровывает шифртекст cipher с использованием AES‑GCM, ключа и nonce.
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
