package bootstrap

import (
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// helper: временный пользовательский конфиг для тестов
func setTempCfg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	// база клиентов хранится в CLIENT_DB_PATH
	db := filepath.Join(dir, "db")
	_ = os.MkdirAll(db, 0o700)
	t.Setenv("CLIENT_DB_PATH", db)
	return dir
}

func TestOpenItemRepo_SuccessAndCleanup(t *testing.T) {
	setTempCfg(t)
	// сохраняем активный логин
	if err := (fsrepo.AuthFSStore{}).SaveLogin("john"); err != nil {
		t.Fatalf("save login: %v", err)
	}
	r, done, err := OpenItemRepo()
	if err != nil {
		t.Fatalf("OpenItemRepo: %v", err)
	}
	// репозиторий должен быть рабочим — попробуем добавить пустую запись
	if _, err := r.AddEncrypted("rec1", nil, nil, nil, nil); err != nil {
		t.Fatalf("AddEncrypted: %v", err)
	}
	if err := done(); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	// повторный вызов cleanup не должен паниковать/падать
	_ = done()
}

func TestOpenItemRepo_ErrorWhenNoLogin(t *testing.T) {
	setTempCfg(t)
	if _, _, err := OpenItemRepo(); err == nil {
		t.Fatalf("expected error when no active login saved")
	}
}

// Доп.кейс: ошибка OpenForUser — когда CLIENT_DB_PATH указывает на обычный файл
func TestOpenItemRepo_OpenForUserFailsWhenClientDBPathIsFile(t *testing.T) {
	dir := setTempCfg(t)
	// Сохраняем активный логин, чтобы пройти первую проверку
	if err := (fsrepo.AuthFSStore{}).SaveLogin("john"); err != nil {
		t.Fatalf("save login: %v", err)
	}
	// Подменим CLIENT_DB_PATH на путь к существующему файлу
	tmpFile := filepath.Join(dir, "not_dir")
	if err := os.WriteFile(tmpFile, []byte("x"), 0o600); err != nil {
		t.Fatalf("prepare tmp file: %v", err)
	}
	t.Setenv("CLIENT_DB_PATH", tmpFile)
	if _, _, err := OpenItemRepo(); err == nil {
		t.Fatalf("expected error when CLIENT_DB_PATH points to file, got nil")
	}
}
