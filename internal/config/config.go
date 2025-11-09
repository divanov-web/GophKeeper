package config

import (
	"flag"
	"os"
	"path/filepath"
	"regexp"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	// Server-side settings
	DatabaseDSN string `env:"DATABASE_URI"`
	AuthSecret  string `env:"AUTH_SECRET"`

	// Shared settings
	BaseURL     string `env:"BASE_URL"`
	EnableHTTPS bool   `env:"ENABLE_HTTPS"`

	// Client-side settings
	ServerURL    string `env:"-"`
	ClientDBPath string `env:"CLIENT_DB_PATH"`
	TokenFile    string `env:"TOKEN_FILE"`
	Version      bool   `env:"-"` // show client version and exit (flag only)
}

func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	_ = env.Parse(cfg)

	// flags работают ТОЛЬКО если переменные из env не заданы
	// Server flags
	flag.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "строка подключения к БД")
	flag.StringVar(&cfg.AuthSecret, "auth-secret", cfg.AuthSecret, "секрет для подписи JWT")
	// Shared/client flags
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "base URL of the GophKeeper server (may be host:port or full URL)")
	flag.BoolVar(&cfg.EnableHTTPS, "https", cfg.EnableHTTPS, "enable HTTPS (client: prefer https scheme for BaseURL)")
	// Client flags
	flag.StringVar(&cfg.ClientDBPath, "client-db", cfg.ClientDBPath, "path to client SQLite DB")
	flag.StringVar(&cfg.TokenFile, "token-file", cfg.TokenFile, "path to auth token file (client)")
	flag.BoolVar(&cfg.Version, "version", cfg.Version, "Show client version and exit")

	flag.Parse()

	// Defaults
	if cfg.AuthSecret == "" {
		cfg.AuthSecret = "dev-secret-key"
	}
	// validate BaseURL: must be in "address:port" (no scheme, no path). Otherwise use default.
	hostPortRe := regexp.MustCompile(`^[A-Za-z0-9\.\-]+:\d{1,5}$`)
	if !hostPortRe.MatchString(cfg.BaseURL) {
		cfg.BaseURL = "localhost:8081"
	}

	if cfg.EnableHTTPS {
		cfg.ServerURL = "https://" + cfg.BaseURL
	} else {
		cfg.ServerURL = "http://" + cfg.BaseURL
	}

	// Fill client defaults if empty
	home, _ := os.UserHomeDir()
	if cfg.ClientDBPath == "" {
		cfg.ClientDBPath = filepath.Join(home, "gkcli.db")
	}
	if cfg.TokenFile == "" {
		cfg.TokenFile = filepath.Join(home, ".gk_token")
	}

	return cfg
}
