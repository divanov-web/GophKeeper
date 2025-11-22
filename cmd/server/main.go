package main

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/handlers"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/repo"
	"GophKeeper/internal/service"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

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

	// context для управления жизненным циклом приложения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	gormDB, err := repo.InitDB(cfg.DatabaseDSN)
	if err != nil {
		sugar.Fatalw("failed to initialize database", "error", err)
	}

	userRepo := repo.NewUserRepository(gormDB)
	userService := service.NewUserService(userRepo)

	fmt.Println(userService.TestData())

	h := handlers.NewHandler(userService, sugar, cfg)

	addr := cfg.BaseURL

	sugar.Infow(
		"Starting server",
		"addr", addr,
	)

	sugar.Infow("Config",
		"BaseURL", cfg.BaseURL,
		"EnableHTTPS", cfg.EnableHTTPS,
		"DatabaseDSN", cfg.DatabaseDSN,
	)

	// создаём http.Server, чтобы иметь возможность выполнить graceful shutdown
	srv := &http.Server{Addr: addr, Handler: h.Router}

	// Регистрируем сигнал о завершении приложения и останавливаем сервер аккуратно
	idleConnsClosed := make(chan struct{})
	go func() {
		// На Windows безопасно слушать только os.Interrupt;
		stopCtx, stop := signal.NotifyContext(ctx, os.Interrupt)
		defer stop()

		// ждём сигнал
		<-stopCtx.Done()

		sugar.Infow("Shutting down...")

		// останавливаем сервер с таймаутом
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			sugar.Errorw("Graceful shutdown error", "error", shutdownErr)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		sugar.Fatalw("Server failed", "error", err)
	}

	// ждём завершения горутины graceful shutdown (если она запускалась)
	<-idleConnsClosed
}
