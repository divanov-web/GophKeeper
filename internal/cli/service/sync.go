package service

import (
	"GophKeeper/internal/cli/api"
	"GophKeeper/internal/cli/model"
	crepo "GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"GophKeeper/internal/config"
	"encoding/json"
	"fmt"
)

// syncRequest/response DTOs соответствуют серверному API /api/items/sync
type syncChange struct {
	ID             string  `json:"id"`
	Name           *string `json:"name,omitempty"`
	FileName       *string `json:"file_name,omitempty"`
	BlobID         *string `json:"blob_id,omitempty"`
	Version        *int64  `json:"version,omitempty"`
	Deleted        *bool   `json:"deleted,omitempty"`
	Resolve        *string `json:"resolve,omitempty"`
	LoginCipher    []byte  `json:"login_cipher,omitempty"`
	LoginNonce     []byte  `json:"login_nonce,omitempty"`
	PasswordCipher []byte  `json:"password_cipher,omitempty"`
	PasswordNonce  []byte  `json:"password_nonce,omitempty"`
	TextCipher     []byte  `json:"text_cipher,omitempty"`
	TextNonce      []byte  `json:"text_nonce,omitempty"`
	CardCipher     []byte  `json:"card_cipher,omitempty"`
	CardNonce      []byte  `json:"card_nonce,omitempty"`
}

type syncRequest struct {
	LastSyncAt string       `json:"last_sync_at,omitempty"`
	Changes    []syncChange `json:"changes"`
}

type appliedDTO struct {
	ID         string `json:"id"`
	NewVersion int64  `json:"new_version"`
}

type conflictDTO struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type syncResponse struct {
	Applied    []appliedDTO  `json:"applied"`
	Conflicts  []conflictDTO `json:"conflicts"`
	ServerTime string        `json:"server_time"`
}

// SyncItemToServer отправляет один item на сервер через /api/items/sync.
// isNew указывает, что запись только что создана локально — в этом случае отправляем version=0.
// Возвращает (applied, newVersion, conflictsText, err).
func SyncItemToServer(cfg *config.Config, item model.Item, isNew bool) (bool, int64, string, error) {
	if cfg == nil {
		return false, 0, "", fmt.Errorf("nil config")
	}
	// токен авторизации
	token, err := (fsrepo.AuthFSStore{}).Load()
	if err != nil {
		return false, 0, "", fmt.Errorf("нет токена авторизации: %w", err)
	}
	// собираем change
	chg := syncChange{ID: item.ID}
	// Версия: для новой записи — 0, иначе локальная версия
	if isNew {
		v := int64(0)
		chg.Version = &v
	} else {
		v := item.Version
		chg.Version = &v
	}
	if item.Name != "" {
		n := item.Name
		chg.Name = &n
	}
	if item.FileName != "" {
		fn := item.FileName
		chg.FileName = &fn
	}
	if item.BlobID != "" {
		bid := item.BlobID
		chg.BlobID = &bid
	}
	// Зашифрованные поля (если есть значения)
	if len(item.LoginCipher) > 0 {
		chg.LoginCipher = item.LoginCipher
	}
	if len(item.LoginNonce) > 0 {
		chg.LoginNonce = item.LoginNonce
	}
	if len(item.PasswordCipher) > 0 {
		chg.PasswordCipher = item.PasswordCipher
	}
	if len(item.PasswordNonce) > 0 {
		chg.PasswordNonce = item.PasswordNonce
	}
	if len(item.TextCipher) > 0 {
		chg.TextCipher = item.TextCipher
	}
	if len(item.TextNonce) > 0 {
		chg.TextNonce = item.TextNonce
	}
	if len(item.CardCipher) > 0 {
		chg.CardCipher = item.CardCipher
	}
	if len(item.CardNonce) > 0 {
		chg.CardNonce = item.CardNonce
	}

	payload := syncRequest{Changes: []syncChange{chg}}
	url := cfg.ServerURL + "/api/items/sync"
	resp, body, err := api.PostJSON(url, payload, token)
	if err != nil {
		return false, 0, "", err
	}
	if resp.StatusCode != 200 {
		return false, 0, "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}
	var sr syncResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return false, 0, "", err
	}
	if len(sr.Applied) > 0 {
		return true, sr.Applied[0].NewVersion, "", nil
	}
	if len(sr.Conflicts) > 0 {
		// конкатенируем причины для краткого вывода
		b, _ := json.Marshal(sr.Conflicts)
		return false, 0, string(b), nil
	}
	return false, 0, "", nil
}

// SyncItemByName загружает локальный item по имени и синхронизирует его на сервере.
func SyncItemByName(cfg *config.Config, r crepo.ItemRepository, name string, isNew bool) (bool, int64, string, error) {
	it, err := r.GetItemByName(name)
	if err != nil {
		return false, 0, "", err
	}
	applied, newVer, conflicts, syncErr := SyncItemToServer(cfg, *it, isNew)
	if syncErr != nil {
		return false, 0, conflicts, syncErr
	}
	if applied {
		// После успешного применения на сервере — зафиксировать серверную версию локально
		if err := r.SetServerVersion(it.ID, newVer); err != nil {
			// Не считаем это фатальной ошибкой отправки: версию можно синхронизировать позже
			// но сообщим вызывающему коду через обёртку ошибки
			return applied, newVer, conflicts, fmt.Errorf("failed to persist server version locally: %w", err)
		}
	}
	return applied, newVer, conflicts, nil
}
