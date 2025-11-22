package handlers

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/service"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ItemHandler обрабатывает синхронизацию записей и загрузку блобов.
type ItemHandler struct {
	ItemService *service.ItemService
	Logger      *zap.SugaredLogger
	Config      *config.Config
}

// NewItemHandler создаёт хендлер items
func NewItemHandler(itemService *service.ItemService, logger *zap.SugaredLogger, cfg *config.Config) *ItemHandler {
	return &ItemHandler{ItemService: itemService, Logger: logger, Config: cfg}
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
	Resolve        *string `json:"resolve,omitempty"` // "client" | "server"
	LoginCipher    []byte  `json:"login_cipher,omitempty"`
	LoginNonce     []byte  `json:"login_nonce,omitempty"`
	PasswordCipher []byte  `json:"password_cipher,omitempty"`
	PasswordNonce  []byte  `json:"password_nonce,omitempty"`
	TextCipher     []byte  `json:"text_cipher,omitempty"`
	TextNonce      []byte  `json:"text_nonce,omitempty"`
	CardCipher     []byte  `json:"card_cipher,omitempty"`
	CardNonce      []byte  `json:"card_nonce,omitempty"`
}

// Ответ синхронизации
type AppliedDTO struct {
	ID         string `json:"id"`
	NewVersion int64  `json:"new_version"`
}

type ConflictDTO struct {
	ID         string      `json:"id"`
	Reason     string      `json:"reason"`
	ServerItem interface{} `json:"server_item,omitempty"`
}

type SyncResponse struct {
	Applied       []AppliedDTO  `json:"applied"`
	Conflicts     []ConflictDTO `json:"conflicts"`
	ServerChanges []any         `json:"server_changes"`
	ServerTime    string        `json:"server_time"`
}

// Sync синхронизация item от клиента
func (h *ItemHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if _, ok := middleware.GetUserIDFromContext(r.Context()); !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req SyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	userID, _ := middleware.GetUserIDFromContext(r.Context())

	// Преобразуем запрос хендлера в сервисный DTO
	var sincePtr *time.Time
	if req.LastSyncAt != "" {
		if t, err := time.Parse(time.RFC3339, req.LastSyncAt); err == nil {
			sincePtr = &t
		}
	}
	svcReq := service.SyncRequest{LastSyncAt: sincePtr, Changes: make([]service.SyncChange, 0, len(req.Changes))}
	for _, ch := range req.Changes {
		svcReq.Changes = append(svcReq.Changes, service.SyncChange{
			ID:             ch.ID,
			Version:        ch.Version,
			Deleted:        ch.Deleted,
			Resolve:        ch.Resolve,
			Name:           ch.Name,
			FileName:       ch.FileName,
			BlobID:         ch.BlobID,
			LoginCipher:    ch.LoginCipher,
			LoginNonce:     ch.LoginNonce,
			PasswordCipher: ch.PasswordCipher,
			PasswordNonce:  ch.PasswordNonce,
			TextCipher:     ch.TextCipher,
			TextNonce:      ch.TextNonce,
			CardCipher:     ch.CardCipher,
			CardNonce:      ch.CardNonce,
		})
	}

	res, _ := h.ItemService.Sync(r.Context(), userID, svcReq)

	// Маппинг ответа сервиса в HTTP DTO
	applied := make([]AppliedDTO, 0, len(res.Applied))
	for _, a := range res.Applied {
		applied = append(applied, AppliedDTO{ID: a.ID, NewVersion: a.NewVersion})
	}
	conflicts := make([]ConflictDTO, 0, len(res.Conflicts))
	for _, c := range res.Conflicts {
		conflicts = append(conflicts, ConflictDTO{ID: c.ID, Reason: c.Reason, ServerItem: c.ServerItem})
	}
	// server_changes пока отдаём как сырые объекты модели
	serverChanges := make([]any, 0, len(res.ServerChanges))
	for _, it := range res.ServerChanges {
		serverChanges = append(serverChanges, map[string]any{
			"id":         it.ID,
			"version":    it.Version,
			"deleted":    it.Deleted,
			"updated_at": it.UpdatedAt.UTC().Format(time.RFC3339),
			"name":       it.Name,
			"file_name":  it.FileName,
			"blob_id":    it.BlobID,
		})
	}

	resp := SyncResponse{
		Applied:       applied,
		Conflicts:     conflicts,
		ServerChanges: serverChanges,
		ServerTime:    res.ServerTime.UTC().Format(time.RFC3339),
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
