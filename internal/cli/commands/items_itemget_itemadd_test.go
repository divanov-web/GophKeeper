package commands

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
	"GophKeeper/internal/config"
)

func TestItems_Run_EmptyAndList(t *testing.T) {
	withTempConfig(t)
	// подготовим пользователя и БД
	_ = (fsrepo.AuthFSStore{}).SaveLogin("john")
	st, _, err := reposqlite.OpenForUser("john")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	// пустой список
	out := withStdoutCapture(t, func() { _ = (itemsCmd{}).Run(&config.Config{}, []string{}) })
	if !strings.Contains(out, "Нет записей") {
		t.Fatalf("expected 'Нет записей', got: %s", out)
	}

	// добавим записи
	if _, err := st.AddEncrypted("A", nil, nil, nil, nil); err != nil {
		t.Fatalf("add A: %v", err)
	}
	if _, err := st.AddEncrypted("B", nil, nil, nil, nil); err != nil {
		t.Fatalf("add B: %v", err)
	}

	out = withStdoutCapture(t, func() { _ = (itemsCmd{}).Run(&config.Config{}, []string{}) })
	if !(strings.Contains(out, "name=A") && strings.Contains(out, "name=B") && strings.Contains(out, "Всего: 2")) {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestItemGet_Run_Success_Usage_NoLogin(t *testing.T) {
	withTempConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("ann")
	st, _, err := reposqlite.OpenForUser("ann")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer st.Close()
	_ = st.Migrate()
	if _, err := st.AddEncrypted("rec1", nil, nil, nil, nil); err != nil {
		t.Fatalf("add: %v", err)
	}

	// успех
	out := withStdoutCapture(t, func() { _ = (itemGetCmd{}).Run(&config.Config{}, []string{"rec1"}) })
	if !strings.Contains(out, "name:      rec1") {
		t.Fatalf("unexpected output: %s", out)
	}

	// ErrUsage
	if err := (itemGetCmd{}).Run(&config.Config{}, []string{}); err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}

	// Нет логина → OpenItemRepo вернёт ошибку
	withTempConfig(t)
	if err := (itemGetCmd{}).Run(&config.Config{}, []string{"rec1"}); err == nil {
		t.Fatalf("expected error without active login")
	}
}

func TestItemAdd_Run_Variants(t *testing.T) {
	withTempConfig(t)
	_ = (fsrepo.AuthFSStore{}).SaveLogin("kate")
	// для успешной синхронизации нужен токен авторизации
	_ = (fsrepo.AuthFSStore{}).Save("tok-123")
	// фазовый сервер для /api/items/sync
	phase := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/items/sync") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		switch phase {
		case 0: // applied
			phase = 1
			_, _ = w.Write([]byte(`{"applied":[{"id":"x","new_version":2}],"conflicts":[],"server_changes":[],"server_time":"2024-01-01T00:00:00Z"}`))
		case 1: // conflict
			phase = 2
			_, _ = w.Write([]byte(`{"applied":[],"conflicts":[{"id":"x","reason":"version_conflict"}],"server_time":"2024-01-01T00:00:00Z"}`))
		default: // non-200 (ошибка)
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// 1) создание без логина/пароля
	out := withStdoutCapture(t, func() { _ = (itemAddCmd{}).Run(cfg, []string{"note1"}) })
	if !(strings.Contains(out, "Created:") && strings.Contains(out, "→ Синхронизация с сервером") && strings.Contains(out, "✓ Синхронизировано. Новая версия: 2")) {
		t.Fatalf("unexpected out1: %s", out)
	}

	// 2) создание c логином и паролем
	out = withStdoutCapture(t, func() { _ = (itemAddCmd{}).Run(cfg, []string{"note2", "user", "pwd"}) })
	if !(strings.Contains(out, "login: <set>") && strings.Contains(out, "password: <set>") && strings.Contains(out, "Конфликт на сервере")) {
		t.Fatalf("unexpected out2: %s", out)
	}

	// 3) ошибка отправки (non-200) → сообщение об ошибке, но команда не падает
	out = withStdoutCapture(t, func() { _ = (itemAddCmd{}).Run(cfg, []string{"note3"}) })
	if !strings.Contains(out, "Ошибка отправки") {
		t.Fatalf("expected sync error message, got: %s", out)
	}

	// 4) валидация аргументов
	if err := (itemAddCmd{}).Run(cfg, []string{}); err != ErrUsage {
		t.Fatalf("expected ErrUsage")
	}
	// пароль без логина → ErrUsage
	if err := (itemAddCmd{}).Run(cfg, []string{"nm", "", "pwd"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage on password without login")
	}

	// sanity: база создана там, где ожидаем
	base := os.Getenv("CLIENT_DB_PATH")
	if _, err := os.Stat(filepath.Join(base, "kate", "client.sqlite")); err != nil {
		t.Fatalf("sqlite for kate missing: %v", err)
	}
}
