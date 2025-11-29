package crypto

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Доп.кейс: Encrypt с ключом неправильной длины
func TestEncrypt_InvalidKeyLen(t *testing.T) {
	_, _, err := Encrypt([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatalf("expected error for invalid key length in Encrypt")
	}
}

// Доп.кейс: Decrypt с ключом неправильной длины
func TestDecrypt_InvalidKeyLen(t *testing.T) {
	if _, err := Decrypt([]byte{1, 2, 3}, []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, []byte("short")); err == nil {
		t.Fatalf("expected error for invalid key length in Decrypt")
	}
}

// Доп.кейс: CLIENT_DB_PATH указывает на файл — keyFilePath/LoadOrCreateKey должны вернуть ошибку
func TestKeyPathAndLoadOrCreateKey_FailsWhenClientDBPathIsFile(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	// создадим файл и подменим CLIENT_DB_PATH на него
	bad := filepath.Join(dir, "not_dir")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatalf("prepare tmp file: %v", err)
	}
	t.Setenv("CLIENT_DB_PATH", bad)

	if _, err := keyFilePath("user"); err == nil {
		t.Fatalf("expected error from keyFilePath when CLIENT_DB_PATH is file")
	}
	if _, err := LoadOrCreateKey("user"); err == nil {
		t.Fatalf("expected error from LoadOrCreateKey when CLIENT_DB_PATH is file")
	}
}
