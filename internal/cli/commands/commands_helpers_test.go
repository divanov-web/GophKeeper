package commands

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// withTempConfig переопределяет пользовательские каталоги на время теста,
// чтобы артефакты (токен/логин/база) создавались в temp.
func withTempConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	db := filepath.Join(dir, "db")
	_ = os.MkdirAll(db, 0o700)
	t.Setenv("CLIENT_DB_PATH", db)
	return dir
}
