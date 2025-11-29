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
	itemService *service.ItemService,
	logger *zap.SugaredLogger,
	config *config.Config,
) *Handler {
	r := chi.NewRouter()

	middleware.SetLogger(logger)

	r.Use(middleware.WithGzip)
	r.Use(middleware.WithLogging)
	r.Use(middleware.WithAuth(config.AuthSecret))

	// Handlers
	userHandler := NewUserHandler(userService, logger, config)
	itemHandler := NewItemHandler(itemService, logger, config)

	// User routes
	r.Post("/api/user/register", userHandler.Register)
	r.Post("/api/user/login", userHandler.Login)
	r.Post("/api/user/test", userHandler.Status)

	// Items/Blobs routes (stubs for now)
	r.Post("/api/items/sync", itemHandler.Sync)
	r.Post("/api/blobs/upload", itemHandler.UploadBlob)

	return &Handler{Router: r}
}
