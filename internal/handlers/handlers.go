package handlers

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/service"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"
)

type Handler struct {
	Router chi.Router
}

// NewHandler разводящий для хендлеров
func NewHandler(
	userService *service.UserService,
	logger *zap.SugaredLogger,
	config *config.Config,
) *Handler {
	r := chi.NewRouter()

	r.Use(middleware.WithGzip)
	r.Use(middleware.WithLogging)
	r.Use(middleware.WithAuth(config.AuthSecret))

	// Handlers
	userHandler := NewUserHandler(userService, logger, config)

	// User routes
	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)
	r.Post("/api/user/test", userHandler.Test)

	return &Handler{Router: r}
}
