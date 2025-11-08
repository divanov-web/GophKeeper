package main

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/handlers"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/repository"
	"GophKeeper/internal/service"
	"context"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func main() {
	cfg := config.NewConfig()

	// создаём предустановленный регистратор zap
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}

	// делаем регистратор SugaredLogger
	sugar := logger.Sugar()
	middleware.SetLogger(sugar) // передаём логгер в middleware
	//сброс буфера логгера
	defer func() {
		if err := logger.Sync(); err != nil {
			sugar.Errorw("Failed to sync logger", "error", err)
		}
	}()

	//context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = ctx

	gormDB, err := repository.InitDB(cfg.DatabaseDSN)
	if err != nil {
		sugar.Fatalw("failed to initialize database", "error", err)
	}

	userRepo := repository.NewUserRepository(gormDB)
	userService := service.NewUserService(userRepo)

	fmt.Println(userService.TestData())

	h := handlers.NewHandler(userService, sugar, cfg)

	sugar.Infow(
		"Starting server",
		"addr", cfg.ServerAddress,
	)

	sugar.Infow("Config",
		"ServerAddress", cfg.ServerAddress,
		"DatabaseDSN", cfg.DatabaseDSN,
	)

	if err := http.ListenAndServe(cfg.ServerAddress, h.Router); err != nil {
		sugar.Fatalw("Server failed", "error", err)
	}
}
