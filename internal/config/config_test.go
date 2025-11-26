package config

import (
	"flag"
	"os"
	"strings"
	"testing"
)

// resetFlagSet создаёт новый FlagSet перед каждым вызовом NewConfig,
// чтобы избежать повторной регистрации одних и тех же флагов между тестами.
func resetFlagSet(t *testing.T) {
	t.Helper()
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	// подавляем вывод парсера флагов в тестах
	flag.CommandLine.SetOutput(os.Stderr)
}

// unsetEnv удаляет переменные окружения из списка
func unsetEnv(t *testing.T, keys ...string) {
	t.Helper()
	for _, k := range keys {
		_ = os.Unsetenv(k)
	}
}

func TestNewConfig_DefaultsWhenEnvEmpty(t *testing.T) {
	t.Setenv("DATABASE_URI", "")
	t.Setenv("AUTH_SECRET", "")
	t.Setenv("BASE_URL", "")
	t.Setenv("ENABLE_HTTPS", "")
	t.Setenv("BLOB_MAX_MB", "")
	t.Setenv("CLIENT_DB_PATH", "")
	t.Setenv("TOKEN_FILE", "")

	resetFlagSet(t)
	cfg := NewConfig()

	if cfg.AuthSecret != "dev-secret-key" {
		t.Fatalf("AuthSecret default expected 'dev-secret-key', got %q", cfg.AuthSecret)
	}
	if cfg.BlobMaxSizeMB != 50 {
		t.Fatalf("BlobMaxSizeMB default expected 50, got %d", cfg.BlobMaxSizeMB)
	}
	if cfg.BaseURL != "localhost:8081" {
		t.Fatalf("BaseURL default expected 'localhost:8081', got %q", cfg.BaseURL)
	}
	if cfg.ServerURL != "http://localhost:8081" {
		t.Fatalf("ServerURL default expected 'http://localhost:8081', got %q", cfg.ServerURL)
	}
	if cfg.ClientDBPath == "" || cfg.TokenFile == "" {
		t.Fatalf("client defaults must be non-empty: ClientDBPath=%q, TokenFile=%q", cfg.ClientDBPath, cfg.TokenFile)
	}
}

func TestNewConfig_BaseURLAndHTTPS(t *testing.T) {
	t.Setenv("BASE_URL", "example.com:443")
	t.Setenv("ENABLE_HTTPS", "true")
	t.Setenv("AUTH_SECRET", "top")
	t.Setenv("BLOB_MAX_MB", "10")

	resetFlagSet(t)
	cfg := NewConfig()

	if cfg.BaseURL != "example.com:443" {
		t.Fatalf("BaseURL expected 'example.com:443', got %q", cfg.BaseURL)
	}
	if cfg.ServerURL != "https://example.com:443" {
		t.Fatalf("ServerURL expected 'https://example.com:443', got %q", cfg.ServerURL)
	}
	if cfg.AuthSecret != "top" {
		t.Fatalf("AuthSecret expected from env 'top', got %q", cfg.AuthSecret)
	}
	if cfg.BlobMaxSizeMB != 10 {
		t.Fatalf("BlobMaxSizeMB expected 10, got %d", cfg.BlobMaxSizeMB)
	}
}

func TestNewConfig_InvalidBaseURLFallback(t *testing.T) {
	// Невалидный BASE_URL (со схемой) должен откатиться на localhost:8081
	t.Setenv("BASE_URL", "http://bad:8080")
	t.Setenv("ENABLE_HTTPS", "false")

	resetFlagSet(t)
	cfg := NewConfig()

	if cfg.BaseURL != "localhost:8081" {
		t.Fatalf("invalid BASE_URL must fallback to 'localhost:8081', got %q", cfg.BaseURL)
	}
	if !strings.HasPrefix(cfg.ServerURL, "http://localhost:8081") {
		t.Fatalf("ServerURL must reflect fallback base, got %q", cfg.ServerURL)
	}
}
