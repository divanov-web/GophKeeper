package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
	"GophKeeper/internal/config"
)

// подготовка окружения пользователя: каталоги, токен, логин и пустая БД
func setupSyncUserEnv(t *testing.T, login string) {
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
	// токен/логин
	_ = (fsrepo.AuthFSStore{}).Save("tok-xyz")
	_ = (fsrepo.AuthFSStore{}).SaveLogin(login)
	// БД пользователя
	st, _, err := reposqlite.OpenForUser(login)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestSync_Run_Applied_PrintSummary(t *testing.T) {
	setupSyncUserEnv(t, "john")
	// сервер: applied=2, server_time задан
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/items/sync") {
			t.Fatalf("bad path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []map[string]any{{"id": "a", "new_version": 2}, {"id": "b", "new_version": 3}},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// перехват вывода
	old := Out
	var buf bytes.Buffer
	Out = &buf
	defer func() { Out = old }()

	err := (syncCmd{}).Run(cfg, []string{})
	if err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	out := buf.String()
	if !(strings.Contains(out, "→ Запуск синхронизации всей базы") && strings.Contains(out, "✓ Применено изменений: 2")) {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestSync_Run_Conflicts_Interactive_ClientThenApplied(t *testing.T) {
	setupSyncUserEnv(t, "ann")
	phase := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch phase {
		case 0: // первый вызов — конфликты
			phase = 1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied":     []any{},
				"conflicts":   []map[string]any{{"id": "x", "reason": "version_conflict"}},
				"server_time": time.Now().UTC().Format(time.RFC3339),
			})
		default: // второй вызов — resolve=client и applied
			// Проверим, что клиент отправил resolve=client
			var req struct {
				Resolve *string `json:"resolve"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Resolve == nil || *req.Resolve != "client" {
				t.Fatalf("expect resolve=client, got %v", req.Resolve)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied":        []map[string]any{{"id": "x", "new_version": 10}},
				"conflicts":      []any{},
				"server_changes": []any{},
				"server_time":    time.Now().UTC().Format(time.RFC3339),
			})
		}
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// stdin: client\n
	restore := setStdin(t, "client\n")
	defer restore()

	old := Out
	var buf bytes.Buffer
	Out = &buf
	defer func() { Out = old }()

	if err := (syncCmd{}).Run(cfg, []string{}); err != nil {
		t.Fatalf("run err: %v", err)
	}
	out := buf.String()
	if !(strings.Contains(out, "Конфликты на сервере") && strings.Contains(out, "Повторная синхронизация (resolve=client)")) {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestSync_Run_Conflicts_Interactive_ServerThenSummary(t *testing.T) {
	setupSyncUserEnv(t, "bob")
	phase := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch phase {
		case 0:
			phase = 1
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied":     []any{},
				"conflicts":   []map[string]any{{"id": "y", "reason": "version_conflict", "server_item": map[string]any{"id": "y", "version": 5, "updated_at": time.Now().UTC().Format(time.RFC3339)}}},
				"server_time": time.Now().UTC().Format(time.RFC3339),
			})
		default:
			var req struct {
				Resolve *string `json:"resolve"`
			}
			_ = json.NewDecoder(r.Body).Decode(&req)
			if req.Resolve == nil || *req.Resolve != "server" {
				t.Fatalf("expect resolve=server, got %v", req.Resolve)
			}
			// При resolve=server RunSyncBatch увеличивает ServerUpserts после применения server_item
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied":        []any{},
				"conflicts":      []any{},
				"server_changes": []any{},
				"server_time":    time.Now().UTC().Format(time.RFC3339),
			})
		}
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	restore := setStdin(t, "server\n")
	defer restore()

	var buf bytes.Buffer
	old := Out
	Out = &buf
	defer func() { Out = old }()

	if err := (syncCmd{}).Run(cfg, []string{}); err != nil {
		t.Fatalf("run err: %v", err)
	}
	out := buf.String()
	// Проверим, что была повторная синхронизация и вывод резюме состоялся (строка о метке сервера или об отсутствии изменений)
	if !(strings.Contains(out, "Повторная синхронизация (resolve=server)") && (strings.Contains(out, "Метка сервера:") || strings.Contains(out, "изменений не применено"))) {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestSync_Run_Conflicts_Interactive_Cancel(t *testing.T) {
	setupSyncUserEnv(t, "kate")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":     []any{},
			"conflicts":   []map[string]any{{"id": "z", "reason": "version_conflict"}},
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	restore := setStdin(t, "cancel\n")
	defer restore()

	var buf bytes.Buffer
	old := Out
	Out = &buf
	defer func() { Out = old }()

	if err := (syncCmd{}).Run(cfg, []string{}); err != nil {
		t.Fatalf("run err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Отменено пользователем") {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestSync_Run_AllAndResolveClient_Flags(t *testing.T) {
	setupSyncUserEnv(t, "nick")
	// Проверим, что last_sync_at = epoch и resolve=client уходит в тело
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LastSyncAt string  `json:"last_sync_at"`
			Resolve    *string `json:"resolve"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.LastSyncAt != "1970-01-01T00:00:00Z" {
			t.Fatalf("expect epoch, got %s", req.LastSyncAt)
		}
		if req.Resolve == nil || *req.Resolve != "client" {
			t.Fatalf("expect resolve=client, got %v", req.Resolve)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []map[string]any{{"id": "x", "new_version": 1}},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	var buf bytes.Buffer
	old := Out
	Out = &buf
	defer func() { Out = old }()

	if err := (syncCmd{}).Run(cfg, []string{"--all", "--resolve=client"}); err != nil {
		t.Fatalf("run err: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "✓ Применено изменений: 1") {
		t.Fatalf("unexpected out: %s", out)
	}
}

func TestSync_Run_ServerErrorPrinted(t *testing.T) {
	setupSyncUserEnv(t, "lena")
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	var buf bytes.Buffer
	old := Out
	Out = &buf
	defer func() { Out = old }()

	if err := (syncCmd{}).Run(cfg, []string{}); err != nil {
		t.Fatalf("run err: %v", err)
	}
	if !strings.Contains(buf.String(), "Ошибка синхронизации") {
		t.Fatalf("expected error printed, got: %s", buf.String())
	}
}

func TestSync_Run_UsageErrors(t *testing.T) {
	// неверное значение resolve
	if err := (syncCmd{}).Run(&config.Config{}, []string{"--resolve=bad"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
}

// setStdin заменяет os.Stdin на временный файл с указанным содержимым.
// Возвращает функцию восстановления исходного stdin.
func setStdin(t *testing.T, content string) func() {
	t.Helper()
	f, err := os.CreateTemp("", "stdin-*")
	if err != nil {
		t.Fatalf("create temp stdin: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write temp stdin: %v", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		t.Fatalf("seek temp stdin: %v", err)
	}
	old := os.Stdin
	os.Stdin = f
	return func() {
		os.Stdin = old
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
	}
}
