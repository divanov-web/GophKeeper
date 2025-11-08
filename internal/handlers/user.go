package handlers

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/service"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"go.uber.org/zap"
)

type UserHandler struct {
	UserService *service.UserService
	Logger      *zap.SugaredLogger
	Config      *config.Config
}

func NewUserHandler(userService *service.UserService, logger *zap.SugaredLogger, config *config.Config) *UserHandler {
	return &UserHandler{
		UserService: userService,
		Logger:      logger,
		Config:      config,
	}
}

type DataResponse struct {
	Result string `json:"result"`
}

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Test для проверки авторизации
func (h *UserHandler) Test(w http.ResponseWriter, r *http.Request) {
	userID, ok := middleware.GetUserIDFromContext(r.Context())
	msg := "anonymous"
	if ok {
		msg = "User ID = " + strconv.FormatInt(userID, 10)
	}
	result := DataResponse{Result: msg}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(result); err != nil {
		http.Error(w, "serialization error", http.StatusInternalServerError)
	}
}

// Register регистрация пользователя
func (h *UserHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.UserService.Register(r.Context(), req.Login, req.Password)
	switch {
	case err == nil:
		_ = middleware.SetLoginCookie(w, user.ID, h.Config.AuthSecret)
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, service.ErrLoginTaken):
		http.Error(w, "login already in use", http.StatusConflict)
	default:
		h.Logger.Errorw("failed to register user", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (h *UserHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	user, err := h.UserService.Login(r.Context(), req.Login, req.Password)
	if err != nil {
		http.Error(w, "invalid login or password", http.StatusUnauthorized)
		return
	}

	if err := middleware.SetLoginCookie(w, user.ID, h.Config.AuthSecret); err != nil {
		h.Logger.Errorw("failed to set cookie", "error", err)
		http.Error(w, "failed to login", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
