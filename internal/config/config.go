package config

import (
	"flag"

	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddress string `env:"RUN_ADDRESS"`
	DatabaseDSN   string `env:"DATABASE_URI"`
	AuthSecret    string `env:"AUTH_SECRET"`
}

func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	_ = env.Parse(cfg) //

	// flags работают ТОЛЬКО если переменные из env не заданы
	flag.StringVar(&cfg.ServerAddress, "a", cfg.ServerAddress, "адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "строка подключения к БД")
	flag.StringVar(&cfg.AuthSecret, "auth-secret", cfg.AuthSecret, "секрет для подписи JWT")
	flag.Parse()

	if cfg.ServerAddress == "" {
		cfg.ServerAddress = "localhost:8081"
	}
	if cfg.AuthSecret == "" {
		cfg.AuthSecret = "dev-secret-key"
	}

	return cfg
}
