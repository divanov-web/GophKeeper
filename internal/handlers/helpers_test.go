package handlers_test

import (
	"GophKeeper/internal/config"
	"GophKeeper/internal/handlers"
	"GophKeeper/internal/middleware"
	"GophKeeper/internal/model"
	"GophKeeper/internal/repo"
	"GophKeeper/internal/service"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// Local light mocks
type hMockItemRepo struct{ mock.Mock }

func (m *hMockItemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	args := m.Called(ctx, userID, since)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *hMockItemRepo) GetByID(ctx context.Context, userID int64, id string) (*model.Item, error) {
	args := m.Called(ctx, userID, id)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *hMockItemRepo) Create(ctx context.Context, it *model.Item) error {
	return m.Called(ctx, it).Error(0)
}
func (m *hMockItemRepo) UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error) {
	args := m.Called(ctx, userID, id, expectedVersion, updates)
	return args.Get(0).(int64), args.Error(1)
}
func (m *hMockItemRepo) ListAll(ctx context.Context, userID int64) ([]model.Item, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.ItemRepository = (*hMockItemRepo)(nil)

type hMockBlobRepo struct{ mock.Mock }

func (m *hMockBlobRepo) CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	args := m.Called(ctx, id, cipher, nonce)
	return args.Bool(0), args.Error(1)
}

var _ repo.BlobRepository = (*hMockBlobRepo)(nil)

type hMockUserRepo struct{ mock.Mock }

func (m *hMockUserRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	args := m.Called(ctx, user)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *hMockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.UserRepository = (*hMockUserRepo)(nil)

func newHandlersTestRouter(t *testing.T) (http.Handler, *config.Config, *hMockItemRepo) {
	t.Helper()
	cfg := &config.Config{AuthSecret: "test-secret", BlobMaxSizeMB: 1}
	logger := zap.NewNop().Sugar()
	ur := &hMockUserRepo{}
	ir := &hMockItemRepo{}
	br := &hMockBlobRepo{}

	userSvc := service.NewUserService(ur)
	itemSvc := service.NewItemService(ir, br, logger)
	h := handlers.NewHandler(userSvc, itemSvc, logger, cfg)
	return h.Router, cfg, ir
}

func addAuth(t *testing.T, req *http.Request, userID int64, secret string) {
	t.Helper()
	rr := httptest.NewRecorder()
	_ = middleware.SetLoginCookie(rr, userID, secret)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
}

// Pinpoint: проверяем, как хендлер маппит server_changes.blob_id (nil/empty/non-empty)
func TestHandlers_Sync_ServerChangesBlobIDMapping(t *testing.T) {
	router, cfg, ir := newHandlersTestRouter(t)

	// Prepare items returned by service (via repo.ListAll on epoch)
	nonEmpty := "BID"
	empty := ""
	now := time.Now().UTC()
	items := []model.Item{
		{ID: "i1", Version: 1, UpdatedAt: now},                    // BlobID nil -> JSON null
		{ID: "i2", Version: 2, UpdatedAt: now, BlobID: &empty},    // empty -> JSON null
		{ID: "i3", Version: 3, UpdatedAt: now, BlobID: &nonEmpty}, // non-empty -> JSON string
	}
	ir.On("ListAll", mock.Anything, int64(9)).Return(items, nil).Once()

	body := bytes.NewBufferString(`{"last_sync_at":"1970-01-01T00:00:00Z","changes":[]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/items/sync", body)
	req.Header.Set("Content-Type", "application/json")
	addAuth(t, req, 9, cfg.AuthSecret)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)

	var resp struct {
		ServerChanges []map[string]any `json:"server_changes"`
	}
	_ = json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&resp)
	if assert.Len(t, resp.ServerChanges, 3) {
		assert.Nil(t, resp.ServerChanges[0]["blob_id"])          // nil
		assert.Nil(t, resp.ServerChanges[1]["blob_id"])          // empty -> nil
		assert.Equal(t, "BID", resp.ServerChanges[2]["blob_id"]) // non-empty -> string
	}

	ir.AssertExpectations(t)
}

// Pinpoint: некорректный last_sync_at не приводит к 400 — хендлер логгирует и продолжает
func TestHandlers_Sync_InvalidLastSyncAtStillOK(t *testing.T) {
	router, cfg, _ := newHandlersTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/items/sync", bytes.NewBufferString(`{"last_sync_at":"bad","changes":[]}`))
	req.Header.Set("Content-Type", "application/json")
	addAuth(t, req, 9, cfg.AuthSecret)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	// базовая форма JSON ответа
	var m map[string]any
	_ = json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&m)
	_, hasApplied := m["applied"]
	_, hasConflicts := m["conflicts"]
	_, hasServerTime := m["server_time"]
	assert.True(t, hasApplied && hasConflicts && hasServerTime)
}
