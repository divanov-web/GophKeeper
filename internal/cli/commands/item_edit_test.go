package commands

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
	"GophKeeper/internal/config"
)

// Базовый успешный сценарий редактирования логина с resolve=client (без интерактива)
func TestItemEdit_Run_Login_ResolveClient_Applied(t *testing.T) {
	withTempConfig(t)
	// активный пользователь и токен
	_ = (fsrepo.AuthFSStore{}).SaveLogin("ivan")
	_ = (fsrepo.AuthFSStore{}).Save("tok-xyz")

	// Готовим пользовательскую БД
	st, _, err := reposqlite.OpenForUser("ivan")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// HTTP сервер для /api/items/sync — всегда applied
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/items/sync") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"applied":[{"id":"x","new_version":2}],"conflicts":[],"server_changes":[],"server_time":"2024-01-01T00:00:00Z"}`))
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// Выполняем: item-edit --resolve=client rec login alice
	out := withStdoutCapture(t, func() {
		_ = (itemEditCmd{}).Run(context.Background(), cfg, []string{"--resolve=client", "rec", "login", "alice"})
	})
	if !(strings.Contains(out, "Created:") && strings.Contains(out, "login: <set>") && strings.Contains(out, "✓ Синхронизировано. Новая версия: 2")) {
		t.Fatalf("unexpected output: %s", out)
	}
}

// Сценарий file: создаём файл, ожидаем загрузку блоба (201) и applied синхронизацию
func TestItemEdit_Run_FileUpload_And_Sync(t *testing.T) {
	withTempConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("mike")
	_ = (fsrepo.AuthFSStore{}).Save("tok-777")

	st, _, err := reposqlite.OpenForUser("mike")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// Подготовим тестовый файл
	tmpFile := filepath.Join(t.TempDir(), "doc.bin")
	_ = os.WriteFile(tmpFile, bytes.Repeat([]byte{1, 2, 3}, 10), 0o600)

	// Тестовый сервер: /api/blobs/upload -> 201, /api/items/sync -> applied
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/api/blobs/upload"):
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id":"BLOB","created":true}`))
		case strings.HasSuffix(r.URL.Path, "/api/items/sync"):
			_, _ = w.Write([]byte(`{"applied":[{"id":"x","new_version":2}],"conflicts":[],"server_changes":[],"server_time":"2024-01-01T00:00:00Z"}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	start := time.Now()
	out := withStdoutCapture(t, func() { _ = (itemEditCmd{}).Run(context.Background(), cfg, []string{"note", "file", tmpFile}) })
	// UploadBlobAsync содержит задержку ~2s, поэтому по длительности поймём, что ожидали его
	if time.Since(start) < 1500*time.Millisecond {
		t.Fatalf("expected to wait for async upload")
	}
	if !(strings.Contains(out, "file: <set>") && (strings.Contains(out, "✓ Файл загружен") || strings.Contains(out, "✓ Файл уже был загружен")) && strings.Contains(out, "✓ Синхронизировано.")) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestItemEdit_Run_UsageErrors(t *testing.T) {
	// мало аргументов
	if err := (itemEditCmd{}).Run(context.Background(), &config.Config{}, []string{"x"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
	// неверный тип
	if err := (itemEditCmd{}).Run(context.Background(), &config.Config{}, []string{"rec", "oops", "val"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage for bad type")
	}
	// неверный resolve
	if err := (itemEditCmd{}).Run(context.Background(), &config.Config{}, []string{"--resolve=bad", "rec", "text", "v"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage for bad resolve")
	}
	// card: не 4 аргумента
	if err := (itemEditCmd{}).Run(context.Background(), &config.Config{}, []string{"rec", "card", "1", "2", "3"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage for card count")
	}
}
