package crypto

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// helper: isolate user config and client db path to temp
func setTempUserEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	base := filepath.Join(dir, "db")
	_ = os.MkdirAll(base, 0o700)
	t.Setenv("CLIENT_DB_PATH", base)
	return dir
}

func TestLoadOrCreateKey_CreateAndReuse(t *testing.T) {
	setTempUserEnv(t)
	// создаст новый ключ
	k1, err := LoadOrCreateKey("john")
	if err != nil {
		t.Fatalf("LoadOrCreateKey create: %v", err)
	}
	if len(k1) != 32 {
		t.Fatalf("key len want 32, got %d", len(k1))
	}
	// повторное получение — тот же ключ
	k2, err := LoadOrCreateKey("john")
	if err != nil {
		t.Fatalf("LoadOrCreateKey reuse: %v", err)
	}
	if string(k1) != string(k2) {
		t.Fatalf("expected same key contents on reuse")
	}
}

func TestLoadOrCreateKey_Errors(t *testing.T) {
	setTempUserEnv(t)
	if _, err := LoadOrCreateKey(""); err == nil {
		t.Fatalf("empty login must fail")
	}
	// подменим файл ключа на неправильной длины
	p, err := keyFilePath("bad")
	if err != nil {
		t.Fatalf("keyFilePath: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte("short"), 0o600); err != nil {
		t.Fatalf("write bad key: %v", err)
	}
	if _, err := LoadOrCreateKey("bad"); err == nil {
		t.Fatalf("invalid key length should error")
	}
}

func TestEncryptDecrypt_RoundTrip_AndErrors(t *testing.T) {
	setTempUserEnv(t)
	key, err := LoadOrCreateKey("alice")
	if err != nil {
		t.Fatal(err)
	}

	cipher, nonce, err := Encrypt([]byte("hello"), key)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	plain, err := Decrypt(cipher, nonce, key)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(plain) != "hello" {
		t.Fatalf("round-trip failed: %q", string(plain))
	}

	// неправильный ключ
	other, _ := LoadOrCreateKey("bob")
	if _, err := Decrypt(cipher, nonce, other); err == nil {
		t.Fatalf("decrypt with wrong key should fail")
	}
	// неверный размер nonce
	if _, err := Decrypt(cipher, []byte{1, 2, 3}, key); err == nil {
		t.Fatalf("decrypt with bad nonce size should fail")
	}
}
