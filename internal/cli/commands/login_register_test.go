package commands

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"GophKeeper/internal/config"
)

// --- login tests ---
func TestLogin_Run_SuccessAndErrors(t *testing.T) {
	withTempConfig(t)

	// HTTP сервер имитирует /api/user/login
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/user/login") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		// успех: 200 + Set-Cookie
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: "tok-123"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL}
	cmd := loginCmd{}
	if err := cmd.Run(context.Background(), cfg, []string{"alice", "secret"}); err != nil {
		t.Fatalf("login should succeed: %v", err)
	}
	// проверим, что токен и логин сохранены
	// токен лежит в %CONFIG%/GophKeeper/auth_token
	var tokenPath string
	if p, err := os.UserConfigDir(); err == nil {
		tokenPath = filepath.Join(p, "GophKeeper", "auth_token")
	}
	b, err := os.ReadFile(tokenPath)
	if err != nil || len(b) == 0 {
		t.Fatalf("auth token not saved: %v", err)
	}
	// для пользователя создаётся база: CLIENT_DB_PATH/<login>/client.sqlite
	base := os.Getenv("CLIENT_DB_PATH")
	if base == "" {
		t.Fatalf("CLIENT_DB_PATH not set in test env")
	}
	if _, err := os.Stat(filepath.Join(base, "alice", "client.sqlite")); err != nil {
		t.Fatalf("user sqlite not created: %v", err)
	}

	// 401 Unauthorized
	ts401 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer ts401.Close()
	cfg401 := &config.Config{ServerURL: ts401.URL}
	if err := cmd.Run(context.Background(), cfg401, []string{"alice", "bad"}); err == nil {
		t.Fatalf("expected error for 401")
	}

	// недостаточно аргументов → ErrUsage
	if err := cmd.Run(context.Background(), cfg, []string{"onlyLogin"}); err == nil {
		t.Fatalf("expected ErrUsage for too few args")
	} else if err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}

	// server 500 → ошибка
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts500.Close()
	cfg500 := &config.Config{ServerURL: ts500.URL}
	if err := cmd.Run(context.Background(), cfg500, []string{"a", "b"}); err == nil {
		t.Fatalf("expected error for 500")
	}
}

// --- register tests ---
func TestRegister_Run_SuccessAndErrors(t *testing.T) {
	withTempConfig(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/user/register") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		http.SetCookie(w, &http.Cookie{Name: "auth_token", Value: "tok-xyz"})
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL}
	cmd := registerCmd{}
	if err := cmd.Run(context.Background(), cfg, []string{"bob", "pwd"}); err != nil {
		t.Fatalf("register should succeed: %v", err)
	}
	// файл логина должен существовать
	cfgDir, _ := os.UserConfigDir()
	if _, err := os.Stat(filepath.Join(cfgDir, "GophKeeper", "last_login")); err != nil {
		t.Fatalf("last_login not saved: %v", err)
	}

	// 409 Conflict
	ts409 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer ts409.Close()
	cfg409 := &config.Config{ServerURL: ts409.URL}
	if err := cmd.Run(context.Background(), cfg409, []string{"bob", "pwd"}); err == nil {
		t.Fatalf("expected conflict error")
	}

	// недостаточно аргументов → ErrUsage
	if err := cmd.Run(context.Background(), cfg, []string{"onlyLogin"}); err == nil {
		t.Fatalf("expected ErrUsage on short args")
	} else if err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}

	// 500
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts500.Close()
	cfg500 := &config.Config{ServerURL: ts500.URL}
	if err := cmd.Run(context.Background(), cfg500, []string{"bob", "pwd"}); err == nil {
		t.Fatalf("expected server error")
	}

	// точечный: убедимся, что база лежит внутри CLIENT_DB_PATH
	base := os.Getenv("CLIENT_DB_PATH")
	ents, _ := os.ReadDir(base)
	if len(ents) == 0 {
		t.Fatalf("no user dir created in CLIENT_DB_PATH")
	}
	// любая найденная директория пользователя должна содержать client.sqlite
	for _, e := range ents {
		if e.IsDir() {
			if _, err := os.Stat(filepath.Join(base, e.Name(), "client.sqlite")); err == nil {
				return
			}
		}
	}
	t.Fatalf("client.sqlite not found in any user directory")
}
