package handlers

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/middleware"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ItemHandler обрабатывает синхронизацию записей и загрузку блобов.
type ItemHandler struct {
	Logger *zap.SugaredLogger
	Config *config.Config
}

func NewItemHandler(logger *zap.SugaredLogger, cfg *config.Config) *ItemHandler {
	return &ItemHandler{Logger: logger, Config: cfg}
}

// SyncRequest — минимальный контракт синхронизации (батч изменений).
type SyncRequest struct {
	LastSyncAt string       `json:"last_sync_at,omitempty"`
	Changes    []ItemChange `json:"changes"`
}

// ItemChange — элемент изменения. Значения могут быть опциональными.
type ItemChange struct {
	ID             string  `json:"id"`
	Name           *string `json:"name,omitempty"`
	FileName       *string `json:"file_name,omitempty"`
	BlobID         *string `json:"blob_id,omitempty"`
	Version        *int64  `json:"version,omitempty"`
	Deleted        *bool   `json:"deleted,omitempty"`
	LoginCipher    []byte  `json:"login_cipher,omitempty"`
	LoginNonce     []byte  `json:"login_nonce,omitempty"`
	PasswordCipher []byte  `json:"password_cipher,omitempty"`
	PasswordNonce  []byte  `json:"password_nonce,omitempty"`
	TextCipher     []byte  `json:"text_cipher,omitempty"`
	TextNonce      []byte  `json:"text_nonce,omitempty"`
	CardCipher     []byte  `json:"card_cipher,omitempty"`
	CardNonce      []byte  `json:"card_nonce,omitempty"`
}

// SyncResponse — заглушка ответа синхронизации.
type SyncResponse struct {
	Applied       []any  `json:"applied"`
	Conflicts     []any  `json:"conflicts"`
	ServerChanges []any  `json:"server_changes"`
	ServerTime    string `json:"server_time"`
}

// Sync — заглушка: принимает батч, возвращает пустой результат синхронизации.
func (h *ItemHandler) Sync(w http.ResponseWriter, r *http.Request) {
	// Требуем авторизацию (middleware уже ставит её, но подстрахуемся)
	if _, ok := middleware.GetUserIDFromContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	resp := SyncResponse{
		Applied:       []any{},
		Conflicts:     []any{},
		ServerChanges: []any{},
		ServerTime:    time.Now().UTC().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// UploadBlob — заглушка загрузки бинарного содержимого.
func (h *ItemHandler) UploadBlob(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserIDFromContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"result": "blob upload stub",
	})
}
