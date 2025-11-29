package service

import (
	"GophKeeper/internal/cli/model"
	crepo "GophKeeper/internal/cli/repo"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"GophKeeper/internal/config"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// --- Мок репозитория для sync ---
type syncMockRepo struct{ mock.Mock }

func (m *syncMockRepo) AddEncrypted(name string, loginCipher, loginNonce, passCipher, passNonce []byte) (string, error) {
	args := m.Called(name, loginCipher, loginNonce, passCipher, passNonce)
	return args.String(0), args.Error(1)
}
func (m *syncMockRepo) ListItems() ([]model.Item, error) {
	args := m.Called()
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *syncMockRepo) GetItemByName(name string) (*model.Item, error) {
	args := m.Called(name)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *syncMockRepo) UpsertLogin(name string, loginCipher, loginNonce []byte) (string, bool, error) {
	args := m.Called(name, loginCipher, loginNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *syncMockRepo) UpsertPassword(name string, passCipher, passNonce []byte) (string, bool, error) {
	args := m.Called(name, passCipher, passNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *syncMockRepo) UpsertText(name string, textCipher, textNonce []byte) (string, bool, error) {
	args := m.Called(name, textCipher, textNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *syncMockRepo) UpsertCard(name string, cardCipher, cardNonce []byte) (string, bool, error) {
	args := m.Called(name, cardCipher, cardNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *syncMockRepo) UpsertFile(name, fileName string, blobCipher, blobNonce []byte) (string, bool, error) {
	args := m.Called(name, fileName, blobCipher, blobNonce)
	return args.String(0), args.Bool(1), args.Error(2)
}
func (m *syncMockRepo) SetServerVersion(id string, version int64) error {
	args := m.Called(id, version)
	return args.Error(0)
}
func (m *syncMockRepo) GetBlobByID(id string) (*model.Blob, error) {
	args := m.Called(id)
	if v, ok := args.Get(0).(*model.Blob); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *syncMockRepo) UpsertFullFromServer(it model.Item) error {
	args := m.Called(it)
	return args.Error(0)
}

var _ crepo.ItemRepository = (*syncMockRepo)(nil)

// тестовая подготовка user config/token
func setupUserEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	t.Setenv("CLIENT_DB_PATH", filepath.Join(dir, "db"))
	_ = (fsrepo.AuthFSStore{}).Save("token-abc")
	_ = (fsrepo.AuthFSStore{}).SaveLogin("user1")
}

func TestSyncItemToServer_Applied(t *testing.T) {
	setupUserEnv(t)
	// сервер вернёт applied
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/items/sync") {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []map[string]any{{"id": "i1", "new_version": 2}},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()

	cfg := &config.Config{ServerURL: ts.URL}
	it := model.Item{ID: "i1", Name: "n", Version: 1}
	applied, newVer, serverVer, conflicts, err := SyncItemToServer(cfg, it, false, nil)
	assert.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, int64(2), newVer)
	assert.Equal(t, int64(0), serverVer)
	assert.Equal(t, "", conflicts)
}

func TestSyncItemToServer_Conflict(t *testing.T) {
	setupUserEnv(t)
	// сервер вернёт конфликт с server_item и версией 5
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":     []any{},
			"conflicts":   []map[string]any{{"id": "i1", "reason": "version_conflict", "server_item": map[string]any{"id": "i1", "version": 5}}},
			"server_time": time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	it := model.Item{ID: "i1", Name: "n", Version: 10}
	applied, newVer, serverVer, conflicts, err := SyncItemToServer(cfg, it, false, nil)
	assert.NoError(t, err)
	assert.False(t, applied)
	assert.Equal(t, int64(0), newVer)
	assert.Equal(t, int64(5), serverVer)
	assert.NotEmpty(t, conflicts)
}

func TestSyncItemToServer_Non200(t *testing.T) {
	setupUserEnv(t)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	it := model.Item{ID: "i1"}
	applied, _, _, _, err := SyncItemToServer(cfg, it, true, nil)
	assert.False(t, applied)
	assert.Error(t, err)
}

func TestSyncItemByName_SuccessAndPersistError(t *testing.T) {
	setupUserEnv(t)
	// сервер: всегда applied с new_version=3
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []map[string]any{{"id": "i2", "new_version": 3}},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// успех
	repo1 := new(syncMockRepo)
	it := &model.Item{ID: "i2", Name: "nm", Version: 2}
	repo1.On("GetItemByName", "nm").Return(it, nil).Once()
	repo1.On("SetServerVersion", "i2", int64(3)).Return(nil).Once()
	applied, newVer, conflicts, err := SyncItemByName(cfg, repo1, "nm", false, nil)
	assert.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, int64(3), newVer)
	assert.Equal(t, "", conflicts)
	repo1.AssertExpectations(t)

	// ошибка сохранения версии
	repo2 := new(syncMockRepo)
	repo2.On("GetItemByName", "nm").Return(it, nil).Once()
	repo2.On("SetServerVersion", "i2", int64(3)).Return(assert.AnError).Once()
	applied, newVer, conflicts, err = SyncItemByName(cfg, repo2, "nm", false, nil)
	assert.True(t, applied)
	assert.Equal(t, int64(3), newVer)
	assert.Equal(t, "", conflicts)
	assert.Error(t, err)
	repo2.AssertExpectations(t)
}

func TestUploadBlobAsync_Success(t *testing.T) {
	setupUserEnv(t)
	// сервер: 201 Created
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"b1","created":true}`))
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	r := new(syncMockRepo)
	r.On("GetBlobByID", "b1").Return(&model.Blob{ID: "b1", Cipher: []byte{1, 2, 3}, Nonce: []byte{9, 9, 9}}, nil).Once()

	ch := UploadBlobAsync(cfg, r, "b1")
	select {
	case res := <-ch:
		assert.NoError(t, res.Err)
		assert.Equal(t, "b1", res.BlobID)
		assert.True(t, res.Created)
		assert.Equal(t, 3, res.Size)
		r.AssertExpectations(t)
	case <-time.After(4 * time.Second):
		t.Fatalf("timeout waiting for upload result")
	}
}

func TestUploadBlobAsync_ErrorStatusesAndNoToken(t *testing.T) {
	// 400 Bad Request case
	{
		setupUserEnv(t)
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "bad", http.StatusBadRequest)
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
	// No token case
	{
		// Do not save token
		dir := t.TempDir()
		if runtime.GOOS == "windows" {
			t.Setenv("APPDATA", dir)
		} else {
			t.Setenv("XDG_CONFIG_HOME", dir)
		}
		t.Setenv("CLIENT_DB_PATH", filepath.Join(dir, "db"))

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusCreated)
		}))
		defer ts.Close()
		cfg := &config.Config{ServerURL: ts.URL}
		r := new(syncMockRepo)
		// этот вызов не должен происходить из-за отсутствия токена, но если произойдёт — защитим мок
		r.On("GetBlobByID", "b1").Return(&model.Blob{ID: "b1"}, nil).Maybe()
		ch := UploadBlobAsync(cfg, r, "b1")
		select {
		case res := <-ch:
			assert.Error(t, res.Err)
		case <-time.After(4 * time.Second):
			t.Fatalf("timeout")
		}
	}
}

func TestRunSyncBatch_ServerChangesApplied(t *testing.T) {
	setupUserEnv(t)
	// сервер отдаёт только server_changes
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// подготовим два server item; во втором blob_id непустой, что вызовет проверку GetBlobByID и постановку в очередь
		now := time.Now().UTC().Format(time.RFC3339)
		resp := map[string]any{
			"applied":   []any{},
			"conflicts": []any{},
			"server_changes": []map[string]any{
				{"id": "s1", "name": "A", "version": 2, "updated_at": now, "blob_id": nil},
				{"id": "s2", "name": "B", "version": 3, "updated_at": now, "blob_id": "BLOB-X", "login_cipher": "AQ==", "login_nonce": "AQ=="},
			},
			"server_time": now,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	r := new(syncMockRepo)
	// список локальных элементов пуст — изменений не отправляем
	r.On("ListItems").Return([]model.Item{}, nil).Once()
	// ожидаем применение обоих server_changes локально и установку версий
	r.On("UpsertFullFromServer", mock.MatchedBy(func(it model.Item) bool { return it.ID == "s1" && it.Version == 2 })).Return(nil).Once()
	r.On("SetServerVersion", "s1", int64(2)).Return(nil).Once()
	r.On("UpsertFullFromServer", mock.MatchedBy(func(it model.Item) bool { return it.ID == "s2" && it.Version == 3 && len(it.LoginCipher) == 1 })).Return(nil).Once()
	r.On("SetServerVersion", "s2", int64(3)).Return(nil).Once()
	// блоб для второго отсутствует локально — пойдёт в очередь
	r.On("GetBlobByID", "BLOB-X").Return((*model.Blob)(nil), assert.AnError).Once()

	res := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
	assert.NoError(t, res.Err)
	assert.Equal(t, 0, res.AppliedCount)
	// В текущей реализации ServerUpserts инкрементируется только при обработке конфликтов (resolve=server),
	// а для server_changes счётчик не увеличивается.
	assert.Equal(t, 0, res.ServerUpserts)
	if assert.Len(t, res.QueuedBlobIDs, 1) {
		assert.Equal(t, "BLOB-X", res.QueuedBlobIDs[0])
	}
	r.AssertExpectations(t)
}

func TestSyncItemToServer_IsNewAndResolveClient(t *testing.T) {
	setupUserEnv(t)
	// проверим, что в запросе version=0 и resolve=client
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload struct {
			Resolve *string          `json:"resolve"`
			Changes []map[string]any `json:"changes"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		if payload.Resolve == nil || *payload.Resolve != "client" {
			t.Fatalf("expect resolve=client, got %v", payload.Resolve)
		}
		if len(payload.Changes) != 1 {
			t.Fatalf("expect 1 change, got %d", len(payload.Changes))
		}
		if v, ok := payload.Changes[0]["version"]; ok {
			switch vv := v.(type) {
			case float64:
				if int64(vv) != 0 {
					t.Fatalf("expect version 0, got %v", vv)
				}
			}
		} else {
			t.Fatalf("version is missing in change")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied":        []map[string]any{{"id": "i3", "new_version": 1}},
			"conflicts":      []any{},
			"server_changes": []any{},
			"server_time":    time.Now().UTC().Format(time.RFC3339),
		})
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	resolve := "client"
	applied, newVer, _, _, err := SyncItemToServer(cfg, model.Item{ID: "i3", Name: "N"}, true, &resolve)
	assert.NoError(t, err)
	assert.True(t, applied)
	assert.Equal(t, int64(1), newVer)
}

func TestRunSyncBatch_AppliedAndResolveServer(t *testing.T) {
	setupUserEnv(t)
	// Сервер будет отдавать ответ с applied, затем ответ с конфликтом и полным server_item
	var phase int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if phase == 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied":        []map[string]any{{"id": "i1", "new_version": 2}},
				"conflicts":      []any{},
				"server_changes": []any{},
				"server_time":    time.Now().UTC().Format(time.RFC3339),
			})
			phase = 1
		} else {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"applied": []any{},
				"conflicts": []map[string]any{{
					"id": "i2", "reason": "version_conflict",
					"server_item": map[string]any{
						"id": "i2", "name": "N", "version": 5, "updated_at": time.Now().UTC().Format(time.RFC3339),
						"blob_id": "BID-1",
					},
				}},
				"server_time": time.Now().UTC().Format(time.RFC3339),
			})
		}
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}

	// Репозиторий: два элемента в списке; для обоих GetItemByName возвращает полные записи
	r := new(syncMockRepo)
	r.On("ListItems").Return([]model.Item{{Name: "A"}, {Name: "B"}}, nil).Twice()
	r.On("GetItemByName", "A").Return(&model.Item{ID: "i1", Name: "A", Version: 1}, nil).Twice()
	r.On("GetItemByName", "B").Return(&model.Item{ID: "i2", Name: "B", Version: 4}, nil).Twice()

	// Для второго прогона (resolve=server) ожидаем применение server_item
	r.On("UpsertFullFromServer", mock.MatchedBy(func(it model.Item) bool { return it.ID == "i2" && it.Version == 5 })).Return(nil).Once()
	r.On("SetServerVersion", "i2", int64(5)).Return(nil).Once()
	// Блоб отсутствует локально — вернём ошибку, чтобы он попал в очередь
	r.On("GetBlobByID", "BID-1").Return((*model.Blob)(nil), assert.AnError).Once()

	// Первый прогон: без resolve, проверим AppliedCount
	res1 := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{})
	assert.NoError(t, res1.Err)
	assert.Equal(t, 1, res1.AppliedCount)

	// Второй прогон: resolve=server, проверим, что применился server_item и blob id попал в очередь
	resol := "server"
	res2 := RunSyncBatch(t.Context(), cfg, r, BatchSyncOptions{Resolve: &resol})
	assert.NoError(t, res2.Err)
	assert.Equal(t, 0, res2.AppliedCount)
	assert.Equal(t, 1, res2.ServerUpserts)
	if assert.Len(t, res2.QueuedBlobIDs, 1) {
		assert.Equal(t, "BID-1", res2.QueuedBlobIDs[0])
	}

	r.AssertExpectations(t)
}
