package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"GophKeeper/internal/cli/model"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"GophKeeper/internal/config"

	"github.com/stretchr/testify/assert"
)

// Дополнительные точечные тесты для добора покрытия слоя service (CLI)

func TestSyncItemToServer_NilConfig(t *testing.T) {
	applied, newVer, serverVer, conflicts, err := SyncItemToServer(nil, model.Item{ID: "x"}, false, nil)
	assert.Error(t, err)
	assert.False(t, applied)
	assert.Equal(t, int64(0), newVer)
	assert.Equal(t, int64(0), serverVer)
	assert.Equal(t, "", conflicts)
}

func TestSyncItemToServer_InvalidJSONResponse(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// намеренно пишем битый JSON
		_, _ = w.Write([]byte("{"))
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL}
	_, _, _, _, err := SyncItemToServer(cfg, model.Item{ID: "i1"}, true, nil)
	assert.Error(t, err)
}

func TestSyncItemToServer_NeutralResponse(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []any{},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	applied, newVer, serverVer, conflicts, err := SyncItemToServer(cfg, model.Item{ID: "i1"}, false, nil)
	assert.NoError(t, err)
	assert.False(t, applied)
	assert.Equal(t, int64(0), newVer)
	assert.Equal(t, int64(0), serverVer)
	assert.Equal(t, "", conflicts)
}

func TestRunSyncBatch_AllFlag_UsesEpoch_And_SkipsBadItem(t *testing.T) {
	setupUserEnv(t)
	// сохраним last_sync_at другое значение, но с opts.All=true должен отправиться epoch
	_ = fsrepo.SaveLastSyncAt("user1", "2020-01-01T00:00:00Z")

	// сервер проверяет тело запроса
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LastSyncAt string           `json:"last_sync_at"`
			Changes    []map[string]any `json:"changes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.LastSyncAt != "1970-01-01T00:00:00Z" {
			t.Fatalf("expected epoch last_sync_at, got %s", req.LastSyncAt)
		}
		if len(req.Changes) != 1 {
			t.Fatalf("expected exactly 1 change (second skipped), got %d", len(req.Changes))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []any{},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	r := new(syncMockRepo)
	// Два элемента в списке
	r.On("ListItems").Return([]model.Item{{Name: "A"}, {Name: "B"}}, nil).Once()
	// Полная запись для A
	r.On("GetItemByName", "A").Return(&model.Item{ID: "id-A", Name: "A", Version: 1}, nil).Once()
	// Ошибка для B — должна быть пропущена
	r.On("GetItemByName", "B").Return((*model.Item)(nil), assert.AnError).Once()

	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{All: true})
	assert.NoError(t, res.Err)
	r.AssertExpectations(t)
}

func TestRunSyncBatch_ListItemsError_Non200_And_JSONError(t *testing.T) {
	setupUserEnv(t)

	// Ошибка ListItems
	{
		cfg := &config.Config{ServerURL: "http://example.invalid"}
		r := new(syncMockRepo)
		r.On("ListItems").Return(nil, assert.AnError).Once()
		res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
		assert.Error(t, res.Err)
		r.AssertExpectations(t)
	}

	// Non-200 от сервера
	{
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "boom", http.StatusInternalServerError)
		}))
		defer ts.Close()
		cfg := &config.Config{ServerURL: ts.URL}
		r := new(syncMockRepo)
		r.On("ListItems").Return([]model.Item{}, nil).Once()
		res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
		assert.Error(t, res.Err)
		r.AssertExpectations(t)
	}

	// JSON decode error
	{
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("{"))
		}))
		defer ts.Close()
		cfg := &config.Config{ServerURL: ts.URL}
		r := new(syncMockRepo)
		r.On("ListItems").Return([]model.Item{}, nil).Once()
		res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
		assert.Error(t, res.Err)
		r.AssertExpectations(t)
	}
}

func TestRunSyncBatch_DefaultsToEpochWhenNoStoredLastSyncAt(t *testing.T) {
	setupUserEnv(t)
	// Удалим файл last_sync_at для пользователя, чтобы сработал дефолт epoch
	// (setupUserEnv сохраняет login, но не last_sync_at)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LastSyncAt string `json:"last_sync_at"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.LastSyncAt != "1970-01-01T00:00:00Z" {
			t.Fatalf("expected epoch by default, got %s", req.LastSyncAt)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied": []any{}, "conflicts": []any{}, "server_changes": []any{},
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	r := new(syncMockRepo)
	r.On("ListItems").Return([]model.Item{}, nil).Once()
	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{All: false})
	assert.NoError(t, res.Err)
	r.AssertExpectations(t)
}

func TestSyncItemToServer_AllFieldsIncluded(t *testing.T) {
	setupUserEnv(t)
	// сервер валидирует, что пришли все возможные поля
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Changes []map[string]any `json:"changes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if len(payload.Changes) != 1 {
			t.Fatalf("expected 1 change")
		}
		ch := payload.Changes[0]
		// проверим наличие ключей скалярных и байтовых
		for _, k := range []string{"name", "file_name", "blob_id", "login_cipher", "login_nonce", "password_cipher", "password_nonce", "text_cipher", "text_nonce", "card_cipher", "card_nonce"} {
			if _, ok := ch[k]; !ok {
				t.Fatalf("missing field %s", k)
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"applied": []map[string]any{{"id": "x", "new_version": 2}}, "conflicts": []any{}, "server_changes": []any{}, "server_time": time.Now().UTC().Format(time.RFC3339)})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	it := model.Item{ID: "x", Name: "N", FileName: "F", BlobID: "B", Version: 1,
		LoginCipher: []byte{1}, LoginNonce: []byte{2}, PasswordCipher: []byte{3}, PasswordNonce: []byte{4},
		TextCipher: []byte{5}, TextNonce: []byte{6}, CardCipher: []byte{7}, CardNonce: []byte{8}}
	applied, newVer, _, _, err := SyncItemToServer(cfg, it, false, nil)
	assert.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, int64(2), newVer)
}

func TestUploadBlobAsync_OK200(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"b4","created":false}`))
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	r := new(syncMockRepo)
	r.On("GetBlobByID", "b4").Return(&model.Blob{ID: "b4", Cipher: []byte{1, 2, 3, 4}, Nonce: []byte{9}}, nil).Once()
	ch := UploadBlobAsync(cfg, r, "b4")
	select {
	case res := <-ch:
		assert.NoError(t, res.Err)
		assert.Equal(t, "b4", res.BlobID)
		assert.False(t, res.Created)
		assert.Equal(t, 4, res.Size)
		r.AssertExpectations(t)
	case <-time.After(4 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRunSyncBatch_Conflicts_NoResolve_ConflictsJSONOnly(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []any{},
			"conflicts":      []map[string]any{{"id": "c1", "reason": "version_conflict"}},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	r := new(syncMockRepo)
	r.On("ListItems").Return([]model.Item{}, nil).Once()
	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
	assert.NoError(t, res.Err)
	assert.NotEmpty(t, res.ConflictsJSON)
	assert.Equal(t, 0, res.ServerUpserts)
	r.AssertExpectations(t)
}

func TestUploadBlobAsync_Status401_And_413(t *testing.T) {
	setupUserEnv(t)
	// 401
	{
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		}))
		defer ts.Close()
		cfg := &config.Config{ServerURL: ts.URL}
		r := new(syncMockRepo)
		r.On("GetBlobByID", "b1").Return(&model.Blob{ID: "b1", Cipher: []byte{1}, Nonce: []byte{2}}, nil).Once()
		ch := UploadBlobAsync(cfg, r, "b1")
		select {
		case res := <-ch:
			assert.Error(t, res.Err)
			r.AssertExpectations(t)
		case <-time.After(4 * time.Second):
			t.Fatalf("timeout")
		}
	}
	// 413
	{
		setupUserEnv(t) // перезапишем токен, если предыдущий сервер закрыт
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		}))
		defer ts.Close()
		cfg := &config.Config{ServerURL: ts.URL}
		r := new(syncMockRepo)
		r.On("GetBlobByID", "b2").Return(&model.Blob{ID: "b2", Cipher: []byte{1, 2}, Nonce: []byte{2, 3}}, nil).Once()
		ch := UploadBlobAsync(cfg, r, "b2")
		select {
		case res := <-ch:
			assert.Error(t, res.Err)
			r.AssertExpectations(t)
		case <-time.After(4 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func TestUploadBlobAsync_UnexpectedStatus(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	r := new(syncMockRepo)
	r.On("GetBlobByID", "b3").Return(&model.Blob{ID: "b3", Cipher: []byte{1}, Nonce: []byte{2}}, nil).Once()
	ch := UploadBlobAsync(cfg, r, "b3")
	select {
	case res := <-ch:
		assert.Error(t, res.Err)
		r.AssertExpectations(t)
	case <-time.After(4 * time.Second):
		t.Fatalf("timeout")
	}
}

func TestRunSyncBatch_UsesStoredLastSyncAt_And_SavesServerTime(t *testing.T) {
	setupUserEnv(t)
	// сохраним last_sync_at пользователя
	stored := "2024-01-02T03:04:05Z"
	_ = fsrepo.SaveLastSyncAt("user1", stored)

	serverTime := time.Now().UTC().Format(time.RFC3339)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			LastSyncAt string `json:"last_sync_at"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.LastSyncAt != stored {
			t.Fatalf("expected last_sync_at from store %s, got %s", stored, req.LastSyncAt)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []any{},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    serverTime,
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	r := new(syncMockRepo)
	r.On("ListItems").Return([]model.Item{}, nil).Once()
	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{All: false})
	assert.NoError(t, res.Err)
	// Проверим, что server_time также сохранён как last_sync_at
	got, err := fsrepo.LoadLastSyncAt("user1")
	assert.NoError(t, err)
	assert.Equal(t, serverTime, got)
	r.AssertExpectations(t)
}

func TestRunSyncBatch_ResolveClient_PropagatesInPayload(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Resolve *string `json:"resolve"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Resolve == nil || *req.Resolve != "client" {
			t.Fatalf("resolve not propagated: %v", req.Resolve)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []any{},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	r := new(syncMockRepo)
	r.On("ListItems").Return([]model.Item{}, nil).Once()
	resol := "client"
	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{Resolve: &resol})
	assert.NoError(t, res.Err)
	r.AssertExpectations(t)
}

func TestSyncItemToServer_ResolveServer_ConflictReturned(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Resolve *string `json:"resolve"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Resolve == nil || *req.Resolve != "server" {
			t.Fatalf("expected resolve=server")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":     []any{},
			"conflicts":   []map[string]any{{"id": "x", "reason": "version_conflict", "server_item": map[string]any{"id": "x", "version": 10}}},
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	resol := "server"
	applied, newVer, serverVer, conflicts, err := SyncItemToServer(cfg, model.Item{ID: "x", Version: 1}, false, &resol)
	assert.NoError(t, err)
	assert.False(t, applied)
	assert.Equal(t, int64(0), newVer)
	assert.Equal(t, int64(10), serverVer)
	assert.NotEmpty(t, conflicts)
}
