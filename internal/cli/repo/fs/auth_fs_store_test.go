package fs

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// setTempCfg перенастраивает пользовательский конфиг‑каталог в temp для изоляции тестов.
func setTempCfg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	return dir
}

func TestAuthFSStore_SaveLoad_Token_TrimsWhitespace(t *testing.T) {
	setTempCfg(t)
	st := AuthFSStore{}
	// Сохранение токена
	if err := st.Save("tok-123\n\n"); err != nil {
		t.Fatalf("save token: %v", err)
	}
	// Дозапишем вручную лишние пробелы в конец файла, чтобы проверить trim
	p, _ := tokenPath()
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = f.WriteString("  \r\n")
	_ = f.Close()

	tok, err := st.Load()
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if tok != "tok-123" {
		t.Fatalf("token not trimmed, got %q", tok)
	}
}

func TestAuthFSStore_Load_TokenMissingOrEmpty(t *testing.T) {
	setTempCfg(t)
	st := AuthFSStore{}
	// отсутствует файл
	if _, err := st.Load(); err == nil {
		t.Fatalf("expected error for missing token file")
	}
	// пустой файл
	p, _ := tokenPath()
	_ = os.MkdirAll(filepath.Dir(p), 0o700)
	_ = os.WriteFile(p, []byte(""), 0o600)
	if _, err := st.Load(); err == nil {
		t.Fatalf("expected error for empty token file")
	}
}

func TestAuthFSStore_SaveLoad_Login_And_Trimming(t *testing.T) {
	setTempCfg(t)
	st := AuthFSStore{}
	if err := st.SaveLogin("alice\n"); err != nil {
		t.Fatalf("save login: %v", err)
	}
	login, err := st.LoadLogin()
	if err != nil {
		t.Fatalf("load login: %v", err)
	}
	if login != "alice" {
		t.Fatalf("login not trimmed, got %q", login)
	}
}

func TestAuthFSStore_SaveLogin_EmptyError(t *testing.T) {
	setTempCfg(t)
	st := AuthFSStore{}
	if err := st.SaveLogin(""); err == nil {
		t.Fatalf("expected error for empty login")
	}
}

func TestSaveLoadLastSyncAt(t *testing.T) {
	setTempCfg(t)
	const user = "bob"
	// пустой логин → ошибка
	if _, err := LoadLastSyncAt(""); err == nil {
		t.Fatalf("expected error on empty login for LoadLastSyncAt")
	}
	if err := SaveLastSyncAt("", "2024-01-01T00:00:00Z"); err == nil {
		t.Fatalf("expected error on empty login for SaveLastSyncAt")
	}
	// success
	ts := "2024-01-01T00:00:00Z\n"
	if err := SaveLastSyncAt(user, ts); err != nil {
		t.Fatalf("save last_sync_at: %v", err)
	}
	got, err := LoadLastSyncAt(user)
	if err != nil {
		t.Fatalf("load last_sync_at: %v", err)
	}
	if got != "2024-01-01T00:00:00Z" {
		t.Fatalf("expected trimmed timestamp, got %q", got)
	}
}
