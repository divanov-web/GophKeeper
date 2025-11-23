package service

import (
	"GophKeeper/internal/model"
	"GophKeeper/internal/repo"
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ItemService инкапсулирует бизнес-логику работы с Item.
type ItemService struct {
	repo     repo.ItemRepository
	blobRepo repo.BlobRepository
	logger   *zap.SugaredLogger
}

// NewItemService создаёт сервис Item.
func NewItemService(r repo.ItemRepository, br repo.BlobRepository, logger *zap.SugaredLogger) *ItemService {
	return &ItemService{repo: r, blobRepo: br, logger: logger}
}

// SaveBlob сохраняет блоб идемпотентно. Возвращает created=true, если блоб был создан.
func (s *ItemService) SaveBlob(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	if s.blobRepo == nil {
		return false, errors.New("blob repository not configured")
	}
	return s.blobRepo.CreateIfAbsent(ctx, id, cipher, nonce)
}

// SyncChange описывает минимальную модель изменения элемента для сервиса.
type SyncChange struct {
	ID      string
	Version *int64
	Deleted *bool
	Resolve *string // "client" | "server" (опционально)
	// Поля для частичных обновлений
	Name     *string
	FileName *string
	BlobID   *string
	// Зашифрованные поля
	LoginCipher    []byte
	LoginNonce     []byte
	PasswordCipher []byte
	PasswordNonce  []byte
	TextCipher     []byte
	TextNonce      []byte
	CardCipher     []byte
	CardNonce      []byte
}

// SyncRequest вход сервиса синхронизации.
type SyncRequest struct {
	LastSyncAt *time.Time
	Changes    []SyncChange
}

// SyncResult результат синхронизации.
type SyncResult struct {
	Applied       []AppliedResult
	Conflicts     []ConflictResult
	ServerChanges []model.Item
	ServerTime    time.Time
}

type AppliedResult struct {
	ID         string `json:"id"`
	NewVersion int64  `json:"new_version"`
}

type ConflictResult struct {
	ID         string      `json:"id"`
	Reason     string      `json:"reason"`
	ServerItem interface{} `json:"server_item,omitempty"`
}

// Sync выполняет синхронизацию items.
func (s *ItemService) Sync(ctx context.Context, userID int64, req SyncRequest) (SyncResult, error) {
	res := SyncResult{
		Applied:       make([]AppliedResult, 0, len(req.Changes)),
		Conflicts:     make([]ConflictResult, 0),
		ServerChanges: []model.Item{},
		ServerTime:    time.Now().UTC(),
	}

	// Основной цикл по изменениям
	for _, ch := range req.Changes {
		// Нормализуем version
		clientVer := int64(-1)
		if ch.Version != nil {
			clientVer = *ch.Version
		}

		// Загружаем текущую запись
		current, err := s.repo.GetByID(ctx, userID, ch.ID)
		if err != nil {
			// Если записи нет
			if errors.Is(err, repoNotFound(err)) {
				// Создание допускается только если version==0
				if clientVer == 0 {
					it := buildItemFromChange(userID, ch)
					it.Version = 1
					it.UpdatedAt = time.Now().UTC()
					if err := s.repo.Create(ctx, &it); err != nil {
						if s.logger != nil {
							s.logger.Errorw("Sync: create item failed",
								"user_id", userID,
								"item_id", ch.ID,
								"error", err,
							)
						}
						// внутренняя ошибка — оформим как конфликт общего вида
						res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "internal_error"})
						continue
					}
					res.Applied = append(res.Applied, AppliedResult{ID: ch.ID, NewVersion: 1})
					continue
				}
				// иначе конфликт: пытаются апдейтить несуществующее
				res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "not_found"})
				continue
			}
			// другая ошибка чтения
			if s.logger != nil {
				s.logger.Errorw("Sync: get item failed",
					"user_id", userID,
					"item_id", ch.ID,
					"error", err,
				)
			}
			res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "internal_error"})
			continue
		}

		// Запись найдена
		if ch.Deleted != nil && *ch.Deleted {
			// Обработка удаления как флага — та же логика OCC
		}

		if ch.Version != nil && *ch.Version == current.Version {
			// Версии совпали — применяем
			updates := buildPatchFromChange(ch, current)
			newVer, err := s.repo.UpdateWithVersion(ctx, userID, ch.ID, current.Version, updates)
			if err != nil {
				if s.logger != nil {
					s.logger.Errorw("Sync: update with version failed",
						"user_id", userID,
						"item_id", ch.ID,
						"expected_version", current.Version,
						"error", err,
					)
				}
				res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "internal_error"})
				continue
			}
			res.Applied = append(res.Applied, AppliedResult{ID: ch.ID, NewVersion: newVer})
			continue
		}

		// Конфликт версий
		// Если передан явный флаг разрешения
		if ch.Resolve != nil {
			switch *ch.Resolve {
			case "client":
				// Применяем поверх серверной версии, независимо от clientVer
				updates := buildPatchFromChange(ch, current)
				newVer, err := s.repo.UpdateWithVersion(ctx, userID, ch.ID, current.Version, updates)
				if err != nil {
					if s.logger != nil {
						s.logger.Errorw("Sync: force client resolve update failed",
							"user_id", userID,
							"item_id", ch.ID,
							"expected_version", current.Version,
							"error", err,
						)
					}
					res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "internal_error"})
					continue
				}
				res.Applied = append(res.Applied, AppliedResult{ID: ch.ID, NewVersion: newVer})
				continue
			case "server":
				res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "version_conflict", ServerItem: minimalServerView(current)})
				continue
			}
		}

		// Попытка авторазрешения: клиент прислал поля только туда, где на сервере пусто
		if onlyFillsEmptyFields(ch, current) {
			updates := buildPatchFromChange(ch, current)
			newVer, err := s.repo.UpdateWithVersion(ctx, userID, ch.ID, current.Version, updates)
			if err != nil {
				if s.logger != nil {
					s.logger.Errorw("Sync: auto-resolve update failed",
						"user_id", userID,
						"item_id", ch.ID,
						"expected_version", current.Version,
						"error", err,
					)
				}
				res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "internal_error"})
				continue
			}
			res.Applied = append(res.Applied, AppliedResult{ID: ch.ID, NewVersion: newVer})
			continue
		}

		// Иначе — конфликт без авторазрешения
		res.Conflicts = append(res.Conflicts, ConflictResult{ID: ch.ID, Reason: "version_conflict", ServerItem: minimalServerView(current)})
	}

	// Server changes since LastSyncAt
	if req.LastSyncAt != nil {
		if items, err := s.repo.GetItemsUpdatedSince(ctx, userID, *req.LastSyncAt); err == nil {
			res.ServerChanges = items
		} else if s.logger != nil {
			s.logger.Errorw("Sync: get items since failed",
				"user_id", userID,
				"since", req.LastSyncAt.UTC().Format(time.RFC3339),
				"error", err,
			)
		}
	}

	res.ServerTime = time.Now().UTC()
	return res, nil
}

// repoNotFound проверяет признак отсутствия записи (gorm.ErrRecordNotFound)
func repoNotFound(err error) error { return gorm.ErrRecordNotFound }

func minimalServerView(it *model.Item) map[string]any {
	var blobID *string
	if it.BlobID != nil {
		s := *it.BlobID
		if s != "" {
			blobID = &s
		}
	}
	return map[string]any{
		"id":         it.ID,
		"version":    it.Version,
		"deleted":    it.Deleted,
		"updated_at": it.UpdatedAt.UTC().Format(time.RFC3339),
		"name":       it.Name,
		"file_name":  it.FileName,
		"blob_id":    blobID,
	}
}

func buildItemFromChange(userID int64, ch SyncChange) model.Item {
	it := model.Item{
		ID:       ch.ID,
		UserID:   userID,
		Name:     valueOr(ch.Name, ""),
		FileName: valueOr(ch.FileName, ""),
	}
	// nullable BlobID
	if ch.BlobID != nil {
		if *ch.BlobID == "" {
			it.BlobID = nil
		} else {
			s := *ch.BlobID
			it.BlobID = &s
		}
	}
	// bytes
	it.LoginCipher = ch.LoginCipher
	it.LoginNonce = ch.LoginNonce
	it.PasswordCipher = ch.PasswordCipher
	it.PasswordNonce = ch.PasswordNonce
	it.TextCipher = ch.TextCipher
	it.TextNonce = ch.TextNonce
	it.CardCipher = ch.CardCipher
	it.CardNonce = ch.CardNonce
	if ch.Deleted != nil {
		it.Deleted = *ch.Deleted
	}
	return it
}

func buildPatchFromChange(ch SyncChange, current *model.Item) map[string]any {
	patch := map[string]any{}
	if ch.Name != nil {
		patch["name"] = *ch.Name
	}
	if ch.FileName != nil {
		patch["file_name"] = *ch.FileName
	}
	if ch.BlobID != nil {
		if *ch.BlobID == "" {
			patch["blob_id"] = nil
		} else {
			patch["blob_id"] = *ch.BlobID
		}
	}
	if ch.Deleted != nil {
		patch["deleted"] = *ch.Deleted
	}
	if ch.LoginCipher != nil {
		patch["login_cipher"] = ch.LoginCipher
	}
	if ch.LoginNonce != nil {
		patch["login_nonce"] = ch.LoginNonce
	}
	if ch.PasswordCipher != nil {
		patch["password_cipher"] = ch.PasswordCipher
	}
	if ch.PasswordNonce != nil {
		patch["password_nonce"] = ch.PasswordNonce
	}
	if ch.TextCipher != nil {
		patch["text_cipher"] = ch.TextCipher
	}
	if ch.TextNonce != nil {
		patch["text_nonce"] = ch.TextNonce
	}
	if ch.CardCipher != nil {
		patch["card_cipher"] = ch.CardCipher
	}
	if ch.CardNonce != nil {
		patch["card_nonce"] = ch.CardNonce
	}
	return patch
}

func onlyFillsEmptyFields(ch SyncChange, cur *model.Item) bool {
	// Проверяем только те поля, которые клиент прислал; если хотя бы одно поле перезаписывает непустое значение — не авторазрешаем
	if ch.Name != nil && cur.Name != "" {
		return false
	}
	if ch.FileName != nil && cur.FileName != "" {
		return false
	}
	if ch.BlobID != nil && cur.BlobID != nil && *cur.BlobID != "" {
		return false
	}
	if ch.LoginCipher != nil && len(cur.LoginCipher) > 0 {
		return false
	}
	if ch.LoginNonce != nil && len(cur.LoginNonce) > 0 {
		return false
	}
	if ch.PasswordCipher != nil && len(cur.PasswordCipher) > 0 {
		return false
	}
	if ch.PasswordNonce != nil && len(cur.PasswordNonce) > 0 {
		return false
	}
	if ch.TextCipher != nil && len(cur.TextCipher) > 0 {
		return false
	}
	if ch.TextNonce != nil && len(cur.TextNonce) > 0 {
		return false
	}
	if ch.CardCipher != nil && len(cur.CardCipher) > 0 {
		return false
	}
	if ch.CardNonce != nil && len(cur.CardNonce) > 0 {
		return false
	}
	if ch.Deleted != nil && cur.Deleted {
		return false
	}
	return true
}

func valueOr[T any](p *T, def T) T {
	if p == nil {
		return def
	}
	return *p
}
