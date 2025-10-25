package config

import (
	"flag"
	"github.com/caarlos0/env/v6"
	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddress  string `env:"RUN_ADDRESS"`
	DatabaseDSN    string `env:"DATABASE_URI"`
	AuthSecret     string `env:"AUTH_SECRET"`
	AccrualAddress string `env:"ACCRUAL_SYSTEM_ADDRESS"`
	SendOrders     bool   `env:"SEND_ORDERS"` //Отправляет заказы в accrual, включать только локально для формирования заглушек.
}

func NewConfig() *Config {
	_ = godotenv.Load()

	cfg := &Config{}
	_ = env.Parse(cfg) //

	// flags работают ТОЛЬКО если переменные из env не заданы
	flag.StringVar(&cfg.ServerAddress, "a", cfg.ServerAddress, "адрес запуска HTTP-сервера")
	flag.StringVar(&cfg.DatabaseDSN, "d", cfg.DatabaseDSN, "строка подключения к БД")
	flag.StringVar(&cfg.AuthSecret, "auth-secret", cfg.AuthSecret, "секрет для подписи JWT")
	flag.StringVar(&cfg.AccrualAddress, "r", cfg.AccrualAddress, "адрес системы начисления баллов accrual")
	flag.BoolVar(&cfg.SendOrders, "send-orders", cfg.SendOrders, "включить отправку заказов в accrual-систему")
	flag.Parse()

	if cfg.ServerAddress == "" {
		cfg.ServerAddress = "localhost:8081"
	}
	if cfg.AuthSecret == "" {
		cfg.AuthSecret = "dev-secret-key"
	}
	if cfg.AccrualAddress == "" {
		cfg.AccrualAddress = "http://localhost:8080"
	}

	return cfg
}
