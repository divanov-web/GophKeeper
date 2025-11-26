package service

import (
	"GophKeeper/internal/cli/api"
	"GophKeeper/internal/cli/model"
	crepo "GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"GophKeeper/internal/config"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
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
	LastSyncAt  string       `json:"last_sync_at,omitempty"`
	Changes     []syncChange `json:"changes"`
	WantMissing bool         `json:"want_missing,omitempty"`
}

type appliedDTO struct {
	ID         string `json:"id"`
	NewVersion int64  `json:"new_version"`
}

type conflictDTO struct {
	ID         string         `json:"id"`
	Reason     string         `json:"reason"`
	ServerItem map[string]any `json:"server_item,omitempty"`
}

type syncResponse struct {
	Applied       []appliedDTO     `json:"applied"`
	Conflicts     []conflictDTO    `json:"conflicts"`
	ServerChanges []map[string]any `json:"server_changes"`
	ServerTime    string           `json:"server_time"`
	MissingItems  []map[string]any `json:"missing_items"`
}

// SyncItemToServer отправляет один item на сервер через /api/items/sync.
// isNew указывает, что запись только что создана локально — в этом случае отправляем version=0.
// Возвращает (applied, newVersion, conflictsText, err).
// resolve: nil (по умолчанию), либо указатель на строку "client" или "server"
// Возвращает: applied, newVersion (если применено), serverVersion (если конфликт и сервер версию вернул), conflictsText, err
func SyncItemToServer(cfg *config.Config, item model.Item, isNew bool, resolve *string) (bool, int64, int64, string, error) {
	if cfg == nil {
		return false, 0, 0, "", fmt.Errorf("nil config")
	}
	// токен авторизации
	token, err := (fsrepo.AuthFSStore{}).Load()
	if err != nil {
		return false, 0, 0, "", fmt.Errorf("нет токена авторизации: %w", err)
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
	if resolve != nil && (*resolve == "client" || *resolve == "server") {
		r := *resolve
		chg.Resolve = &r
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
		return false, 0, 0, "", err
	}
	if resp.StatusCode != 200 {
		return false, 0, 0, "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}
	var sr syncResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return false, 0, 0, "", err
	}
	if len(sr.Applied) > 0 {
		return true, sr.Applied[0].NewVersion, 0, "", nil
	}
	if len(sr.Conflicts) > 0 {
		// конкатенируем причины для краткого вывода
		// попробуем вытащить серверную версию из первого конфликта, если она есть
		var serverVer int64
		if si := sr.Conflicts[0].ServerItem; si != nil {
			if v, ok := si["version"]; ok {
				switch vv := v.(type) {
				case float64:
					serverVer = int64(vv)
				case int64:
					serverVer = vv
				case json.Number:
					if iv, err := vv.Int64(); err == nil {
						serverVer = iv
					}
				}
			}
		}
		b, _ := json.Marshal(sr.Conflicts)
		return false, 0, serverVer, string(b), nil
	}
	return false, 0, 0, "", nil
}

// SyncItemByName загружает локальный item по имени и синхронизирует его на сервере.
func SyncItemByName(cfg *config.Config, r crepo.ItemRepository, name string, isNew bool, resolve *string) (bool, int64, string, error) {
	it, err := r.GetItemByName(name)
	if err != nil {
		return false, 0, "", err
	}
	applied, newVer, _, conflicts, syncErr := SyncItemToServer(cfg, *it, isNew, resolve)
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
		return applied, newVer, conflicts, nil
	}
	// Если не применено и запрошено resolve=server — применяем полный server_item (если пришёл) и выравниваем версию
	if resolve != nil && *resolve == "server" && conflicts != "" {
		var confs []conflictDTO
		if err := json.Unmarshal([]byte(conflicts), &confs); err == nil {
			// Соберём blob_id для последующей догрузки (если локально отсутствуют)
			pendingBlobIDs := make(map[string]struct{})
			for _, c := range confs {
				if c.ServerItem == nil {
					continue
				}
				// построим client model.Item из server_item
				sit := c.ServerItem
				// обязательные поля
				sid, _ := sit["id"].(string)
				sname, _ := sit["name"].(string)
				sfile, _ := sit["file_name"].(string)
				// версия
				var sver int64
				switch v := sit["version"].(type) {
				case float64:
					sver = int64(v)
				case int64:
					sver = v
				case json.Number:
					if iv, err := v.Int64(); err == nil {
						sver = iv
					}
				}
				// updated_at
				updUnix := time.Now().Unix()
				if us, ok := sit["updated_at"].(string); ok {
					if t, err := time.Parse(time.RFC3339, us); err == nil {
						updUnix = t.Unix()
					}
				}
				// blob_id
				blobID := ""
				if braw, ok := sit["blob_id"]; ok && braw != nil {
					switch bv := braw.(type) {
					case string:
						blobID = bv
					case *string:
						if bv != nil {
							blobID = *bv
						}
					}
				}
				// шифр-поля: сервер отдаёт []byte как base64-строки в JSON; поддержим также []byte на всякий случай
				toBytes := func(m map[string]any, key string) []byte {
					if val, ok := m[key]; ok && val != nil {
						switch vv := val.(type) {
						case string:
							// base64 → []byte
							b, err := base64.StdEncoding.DecodeString(vv)
							if err == nil {
								return b
							}
						case []byte:
							return vv
						}
					}
					return nil
				}
				itm := model.Item{
					ID:             sid,
					Name:           sname,
					CreatedAt:      updUnix,
					UpdatedAt:      updUnix,
					Version:        sver,
					Deleted:        false,
					FileName:       sfile,
					BlobID:         blobID,
					LoginCipher:    toBytes(sit, "login_cipher"),
					LoginNonce:     toBytes(sit, "login_nonce"),
					PasswordCipher: toBytes(sit, "password_cipher"),
					PasswordNonce:  toBytes(sit, "password_nonce"),
					TextCipher:     toBytes(sit, "text_cipher"),
					TextNonce:      toBytes(sit, "text_nonce"),
					CardCipher:     toBytes(sit, "card_cipher"),
					CardNonce:      toBytes(sit, "card_nonce"),
				}
				// deleted
				if del, ok := sit["deleted"].(bool); ok {
					itm.Deleted = del
				}
				// применяем локально
				if sid != "" {
					_ = r.UpsertFullFromServer(itm)
					// выравниваем версию
					_ = r.SetServerVersion(itm.ID, itm.Version)
					// проверим blob_id
					if blobID != "" {
						if _, err := r.GetBlobByID(blobID); err != nil {
							pendingBlobIDs[blobID] = struct{}{}
						}
					}
				}
			}
			if len(pendingBlobIDs) > 0 {
				ids := make([]string, 0, len(pendingBlobIDs))
				for id := range pendingBlobIDs {
					ids = append(ids, id)
				}
				QueueBlobsForDownload(ids)
			}
		}
	}
	return applied, newVer, conflicts, nil
}

// UploadResult результат асинхронной загрузки блоба
type UploadResult struct {
	BlobID  string
	Created bool // true, если блоб создан впервые (201), false — уже существовал (200)
	Size    int  // размер cipher
	Err     error
}

// UploadBlobAsync запускает асинхронную загрузку блоба на сервер в отдельной горутине.
// Возвращает канал, в который по завершении будет отправлен UploadResult.
func UploadBlobAsync(cfg *config.Config, r crepo.ItemRepository, blobID string) <-chan UploadResult {
	out := make(chan UploadResult, 1)
	go func() {
		defer close(out)
		// искусственная задержка
		time.Sleep(2 * time.Second)

		// Загружаем токен
		token, err := (fsrepo.AuthFSStore{}).Load()
		if err != nil {
			out <- UploadResult{BlobID: blobID, Err: fmt.Errorf("нет токена авторизации: %w", err)}
			return
		}
		// Берём блоб из локальной БД
		b, err := r.GetBlobByID(blobID)
		if err != nil {
			out <- UploadResult{BlobID: blobID, Err: err}
			return
		}
		url := cfg.ServerURL + "/api/blobs/upload"
		resp, body, err := api.PostMultipartBlob(url, b.ID, b.Cipher, b.Nonce, token)
		if err != nil {
			out <- UploadResult{BlobID: blobID, Err: err}
			return
		}
		created := false
		switch resp.StatusCode {
		case 200:
			created = false
		case 201:
			created = true
		case 400, 401, 413:
			out <- UploadResult{BlobID: blobID, Err: fmt.Errorf("upload failed: %s", string(body))}
			return
		default:
			out <- UploadResult{BlobID: blobID, Err: fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))}
			return
		}
		out <- UploadResult{BlobID: blobID, Created: created, Size: len(b.Cipher), Err: nil}
	}()
	return out
}

// QueueBlobsForDownload — заглушка очереди на последующую догрузку блобов по их id.
// На этом шаге просто логически фиксируем список; сетевые вызовы будут добавлены позже.
func QueueBlobsForDownload(ids []string) {
	// TODO: реализовать очередь/задачи на скачивание блобов с сервера.
	// Пока что ничего не делаем.
	_ = ids
}

// BatchSyncOptions задаёт параметры пакетной синхронизации
type BatchSyncOptions struct {
	All     bool    // если true — использовать last_sync_at с эпохи
	Resolve *string // опциональная стратегия для всех конфликтов: client|server
}

// BatchSyncResult результат пакетной синхронизации
type BatchSyncResult struct {
	AppliedCount  int
	ServerUpserts int
	ConflictsJSON string
	QueuedBlobIDs []string
	ServerTime    string
	Err           error
}

// RunSyncBatch выполняет пакетную синхронизацию всех локальных записей с сервером.
// Простой вариант: отправляем все локальные записи как changes; также указываем last_sync_at
// (эпоха при opts.All или прочитанное из конфигурации пользователя), применяем server_changes локально.
func RunSyncBatch(ctx context.Context, cfg *config.Config, r crepo.ItemRepository, opts BatchSyncOptions) BatchSyncResult {
	// Загрузка токена
	token, err := (fsrepo.AuthFSStore{}).Load()
	if err != nil {
		return BatchSyncResult{Err: fmt.Errorf("нет токена авторизации: %w", err)}
	}
	// Определим логин пользователя (для хранения last_sync_at в пользовательском конфиге)
	login, lerr := (fsrepo.AuthFSStore{}).LoadLogin()
	if lerr != nil {
		return BatchSyncResult{Err: fmt.Errorf("нет активного пользователя: %w", lerr)}
	}

	// last_sync_at
	lastSyncAt := ""
	if opts.All {
		lastSyncAt = "1970-01-01T00:00:00Z"
	} else {
		if v, err := fsrepo.LoadLastSyncAt(login); err == nil && v != "" {
			lastSyncAt = v
		} else {
			lastSyncAt = "1970-01-01T00:00:00Z"
		}
	}

	// Соберём локальные элементы для changes
	items, err := r.ListItems()
	if err != nil {
		return BatchSyncResult{Err: err}
	}
	changes := make([]syncChange, 0, len(items))
	for _, meta := range items {
		// Берём полную запись (включая зашифрованные поля)
		it, gerr := r.GetItemByName(meta.Name)
		if gerr != nil {
			// пропустим одну запись, но продолжим остальные
			continue
		}
		ch := syncChange{ID: it.ID}
		v := it.Version
		ch.Version = &v
		if opts.Resolve != nil && (*opts.Resolve == "client" || *opts.Resolve == "server") {
			rv := *opts.Resolve
			ch.Resolve = &rv
		}
		if it.Name != "" {
			n := it.Name
			ch.Name = &n
		}
		if it.FileName != "" {
			fn := it.FileName
			ch.FileName = &fn
		}
		if it.BlobID != "" {
			bid := it.BlobID
			ch.BlobID = &bid
		}
		if len(it.LoginCipher) > 0 {
			ch.LoginCipher = it.LoginCipher
		}
		if len(it.LoginNonce) > 0 {
			ch.LoginNonce = it.LoginNonce
		}
		if len(it.PasswordCipher) > 0 {
			ch.PasswordCipher = it.PasswordCipher
		}
		if len(it.PasswordNonce) > 0 {
			ch.PasswordNonce = it.PasswordNonce
		}
		if len(it.TextCipher) > 0 {
			ch.TextCipher = it.TextCipher
		}
		if len(it.TextNonce) > 0 {
			ch.TextNonce = it.TextNonce
		}
		if len(it.CardCipher) > 0 {
			ch.CardCipher = it.CardCipher
		}
		if len(it.CardNonce) > 0 {
			ch.CardNonce = it.CardNonce
		}
		changes = append(changes, ch)
	}

	payload := syncRequest{Changes: changes}
	if lastSyncAt != "" {
		payload.LastSyncAt = lastSyncAt
	}
	// Запросим у сервера записи, которых нет у клиента, только при полной синхронизации
	payload.WantMissing = opts.All
	url := cfg.ServerURL + "/api/items/sync"
	resp, body, err := api.PostJSON(url, payload, token)
	if err != nil {
		return BatchSyncResult{Err: err}
	}
	if resp.StatusCode != 200 {
		return BatchSyncResult{Err: fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))}
	}
	var sr syncResponse
	if err := json.Unmarshal(body, &sr); err != nil {
		return BatchSyncResult{Err: err}
	}

	res := BatchSyncResult{}
	// Applied count
	res.AppliedCount = len(sr.Applied)

	// Обработка конфликтов
	if len(sr.Conflicts) > 0 {
		// Если resolve=server — применим полные server_item (если они присутствуют) локально
		if opts.Resolve != nil && *opts.Resolve == "server" {
			pending := map[string]struct{}{}
			for _, c := range sr.Conflicts {
				if c.ServerItem == nil {
					continue
				}
				sit := c.ServerItem
				sid, _ := sit["id"].(string)
				sname, _ := sit["name"].(string)
				sfile, _ := sit["file_name"].(string)
				var sver int64
				if v, ok := sit["version"]; ok {
					switch vv := v.(type) {
					case float64:
						sver = int64(vv)
					case int64:
						sver = vv
					case json.Number:
						if iv, e := vv.Int64(); e == nil {
							sver = iv
						}
					}
				}
				// updated_at
				updUnix := time.Now().Unix()
				if us, ok := sit["updated_at"].(string); ok {
					if t, e := time.Parse(time.RFC3339, us); e == nil {
						updUnix = t.Unix()
					}
				}
				// blob_id
				blobID := ""
				if braw, ok := sit["blob_id"]; ok && braw != nil {
					switch bv := braw.(type) {
					case string:
						blobID = bv
					case *string:
						if bv != nil {
							blobID = *bv
						}
					}
				}
				// helper для []byte
				toBytes := func(m map[string]any, key string) []byte {
					if val, ok := m[key]; ok && val != nil {
						switch vv := val.(type) {
						case string:
							if b, e := base64.StdEncoding.DecodeString(vv); e == nil {
								return b
							}
						case []byte:
							return vv
						}
					}
					return nil
				}
				itm := model.Item{
					ID:             sid,
					Name:           sname,
					CreatedAt:      updUnix,
					UpdatedAt:      updUnix,
					Version:        sver,
					Deleted:        false,
					FileName:       sfile,
					BlobID:         blobID,
					LoginCipher:    toBytes(sit, "login_cipher"),
					LoginNonce:     toBytes(sit, "login_nonce"),
					PasswordCipher: toBytes(sit, "password_cipher"),
					PasswordNonce:  toBytes(sit, "password_nonce"),
					TextCipher:     toBytes(sit, "text_cipher"),
					TextNonce:      toBytes(sit, "text_nonce"),
					CardCipher:     toBytes(sit, "card_cipher"),
					CardNonce:      toBytes(sit, "card_nonce"),
				}
				if del, ok := sit["deleted"].(bool); ok {
					itm.Deleted = del
				}
				if sid != "" {
					_ = r.UpsertFullFromServer(itm)
					_ = r.SetServerVersion(itm.ID, itm.Version)
					if blobID != "" {
						if _, e := r.GetBlobByID(blobID); e != nil {
							pending[blobID] = struct{}{}
						}
					}
				}
			}
			if len(pending) > 0 {
				res.QueuedBlobIDs = make([]string, 0, len(pending))
				for id := range pending {
					res.QueuedBlobIDs = append(res.QueuedBlobIDs, id)
				}
				QueueBlobsForDownload(res.QueuedBlobIDs)
			}
		}
		if b, e := json.Marshal(sr.Conflicts); e == nil {
			res.ConflictsJSON = string(b)
		}
	}

	// По ТЗ: server_changes с неполными данными НЕ сохраняем локально.

	// Обработка missing_items (полные снимки записей, отсутствующих у клиента)
	if len(sr.MissingItems) > 0 {
		pending := map[string]struct{}{}
		for _, sit := range sr.MissingItems {
			sid, _ := sit["id"].(string)
			sname, _ := sit["name"].(string)
			sfile, _ := sit["file_name"].(string)
			var sver int64
			if v, ok := sit["version"]; ok {
				switch vv := v.(type) {
				case float64:
					sver = int64(vv)
				case int64:
					sver = vv
				case json.Number:
					if iv, e := vv.Int64(); e == nil {
						sver = iv
					}
				}
			}
			// updated_at
			updUnix := time.Now().Unix()
			if us, ok := sit["updated_at"].(string); ok {
				if t, e := time.Parse(time.RFC3339, us); e == nil {
					updUnix = t.Unix()
				}
			}
			// blob_id
			blobID := ""
			if braw, ok := sit["blob_id"]; ok && braw != nil {
				switch bv := braw.(type) {
				case string:
					blobID = bv
				case *string:
					if bv != nil {
						blobID = *bv
					}
				}
			}
			// helper для []byte
			toBytes := func(m map[string]any, key string) []byte {
				if val, ok := m[key]; ok && val != nil {
					switch vv := val.(type) {
					case string:
						if b, e := base64.StdEncoding.DecodeString(vv); e == nil {
							return b
						}
					case []byte:
						return vv
					}
				}
				return nil
			}
			itm := model.Item{
				ID:             sid,
				Name:           sname,
				CreatedAt:      updUnix,
				UpdatedAt:      updUnix,
				Version:        sver,
				Deleted:        false,
				FileName:       sfile,
				BlobID:         blobID,
				LoginCipher:    toBytes(sit, "login_cipher"),
				LoginNonce:     toBytes(sit, "login_nonce"),
				PasswordCipher: toBytes(sit, "password_cipher"),
				PasswordNonce:  toBytes(sit, "password_nonce"),
				TextCipher:     toBytes(sit, "text_cipher"),
				TextNonce:      toBytes(sit, "text_nonce"),
				CardCipher:     toBytes(sit, "card_cipher"),
				CardNonce:      toBytes(sit, "card_nonce"),
			}
			if del, ok := sit["deleted"].(bool); ok {
				itm.Deleted = del
			}
			if sid != "" {
				_ = r.UpsertFullFromServer(itm)
				_ = r.SetServerVersion(itm.ID, itm.Version)
				if blobID != "" {
					if _, e := r.GetBlobByID(blobID); e != nil {
						pending[blobID] = struct{}{}
					}
				}
				res.ServerUpserts++
			}
		}
		if len(pending) > 0 {
			res.QueuedBlobIDs = make([]string, 0, len(pending))
			for id := range pending {
				res.QueuedBlobIDs = append(res.QueuedBlobIDs, id)
			}
			QueueBlobsForDownload(res.QueuedBlobIDs)
		}
	}

	// Сохраним server_time как last_sync_at в конфиг пользователя
	if sr.ServerTime != "" {
		_ = fsrepo.SaveLastSyncAt(login, sr.ServerTime)
		res.ServerTime = sr.ServerTime
	}
	return res
}
