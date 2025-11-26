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
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

// Minimal mocks
type itemMockItemRepo struct{ mock.Mock }

func (m *itemMockItemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	args := m.Called(ctx, userID, since)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *itemMockItemRepo) GetByID(ctx context.Context, userID int64, id string) (*model.Item, error) {
	args := m.Called(ctx, userID, id)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *itemMockItemRepo) Create(ctx context.Context, it *model.Item) error {
	return m.Called(ctx, it).Error(0)
}
func (m *itemMockItemRepo) UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error) {
	args := m.Called(ctx, userID, id, expectedVersion, updates)
	return args.Get(0).(int64), args.Error(1)
}
func (m *itemMockItemRepo) ListAll(ctx context.Context, userID int64) ([]model.Item, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.ItemRepository = (*itemMockItemRepo)(nil)

type itemMockBlobRepo struct{ mock.Mock }

func (m *itemMockBlobRepo) CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	args := m.Called(ctx, id, cipher, nonce)
	return args.Bool(0), args.Error(1)
}

var _ repo.BlobRepository = (*itemMockBlobRepo)(nil)

type itemMockUserRepo struct{ mock.Mock }

func (m *itemMockUserRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	args := m.Called(ctx, user)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *itemMockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.UserRepository = (*itemMockUserRepo)(nil)

func newItemTestRouter(t *testing.T) (http.Handler, *config.Config, *itemMockItemRepo, *itemMockBlobRepo) {
	t.Helper()
	cfg := &config.Config{AuthSecret: "test-secret", BlobMaxSizeMB: 1}
	logger := zap.NewNop().Sugar()
	ur := &itemMockUserRepo{}
	ir := &itemMockItemRepo{}
	br := &itemMockBlobRepo{}

	userSvc := service.NewUserService(ur)
	itemSvc := service.NewItemService(ir, br, logger)
	h := handlers.NewHandler(userSvc, itemSvc, logger, cfg)
	return h.Router, cfg, ir, br
}

func addItemAuthCookie(t *testing.T, req *http.Request, userID int64, secret string) {
	t.Helper()
	rr := httptest.NewRecorder()
	_ = middleware.SetLoginCookie(rr, userID, secret)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
}

func TestItem_Sync_Unauthorized(t *testing.T) {
	router, _, _, _ := newItemTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/items/sync", strings.NewReader(`{"changes":[]}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestItem_Sync_EmptyOK(t *testing.T) {
	router, cfg, _, _ := newItemTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/items/sync", strings.NewReader(`{"changes":[]}`))
	req.Header.Set("Content-Type", "application/json")
	addItemAuthCookie(t, req, 5, cfg.AuthSecret)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	// response must be JSON with required fields
	body := rr.Body.Bytes()
	assert.Contains(t, string(body), "\"applied\"")
	assert.Contains(t, string(body), "\"conflicts\"")
	assert.Contains(t, string(body), "\"server_changes\"")
	assert.Contains(t, string(body), "\"server_time\"")
}

// --- UploadBlob tests ---

// helper to build multipart body
func makeMultipart(t *testing.T, fields map[string]string, files map[string][]byte) (string, *bytes.Buffer) {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for name, data := range files {
		fw, _ := w.CreateFormFile(name, name)
		_, _ = fw.Write(data)
	}
	_ = w.Close()
	return w.FormDataContentType(), body
}

func TestItem_UploadBlob_Unauthorized(t *testing.T) {
	router, _, _, _ := newItemTestRouter(t)
	ct, body := makeMultipart(t, map[string]string{"id": "b1", "nonce": "AAA="}, map[string][]byte{"cipher": []byte{1, 2, 3}})
	req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
	req.Header.Set("Content-Type", ct)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestItem_UploadBlob_ValidationErrors(t *testing.T) {
	router, cfg, _, _ := newItemTestRouter(t)

	// missing id
	{
		ct, body := makeMultipart(t, map[string]string{"nonce": "AAA="}, map[string][]byte{"cipher": []byte{1}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
	// missing cipher
	{
		ct, body := makeMultipart(t, map[string]string{"id": "b1", "nonce": "AAA="}, map[string][]byte{})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
	// invalid base64 nonce string
	{
		ct, body := makeMultipart(t, map[string]string{"id": "b1", "nonce": "*bad*"}, map[string][]byte{"cipher": []byte{1}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
	// missing nonce entirely
	{
		ct, body := makeMultipart(t, map[string]string{"id": "b1"}, map[string][]byte{"cipher": []byte{1}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
	// empty nonce via file
	{
		ct, body := makeMultipart(t, map[string]string{"id": "b1"}, map[string][]byte{"cipher": []byte{1}, "nonce": []byte{}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}
}

func TestItem_UploadBlob_PayloadTooLargeAndSuccess(t *testing.T) {
	router, cfg, _, br := newItemTestRouter(t)

	// too large cipher (> BlobMaxSizeMB)
	{
		big := bytes.Repeat([]byte{1}, (cfg.BlobMaxSizeMB*1024*1024)+1)
		ct, body := makeMultipart(t, map[string]string{"id": "b1", "nonce": "AQ=="}, map[string][]byte{"cipher": big})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	}

	// created=true -> 201
	{
		br.ExpectedCalls = nil
		br.On("CreateIfAbsent", mock.Anything, "bid1", mock.Anything, mock.Anything).Return(true, nil).Once()
		ct, body := makeMultipart(t, map[string]string{"id": "bid1", "nonce": "AQ=="}, map[string][]byte{"cipher": []byte{1, 2, 3}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusCreated, rr.Code)
		br.AssertExpectations(t)
	}

	// created=false -> 200
	{
		br.ExpectedCalls = nil
		br.On("CreateIfAbsent", mock.Anything, "bid2", mock.Anything, mock.Anything).Return(false, nil).Once()
		ct, body := makeMultipart(t, map[string]string{"id": "bid2", "nonce": "AQ=="}, map[string][]byte{"cipher": []byte{1}})
		req := httptest.NewRequest(http.MethodPost, "/api/blobs/upload", body)
		req.Header.Set("Content-Type", ct)
		addItemAuthCookie(t, req, 5, cfg.AuthSecret)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		br.AssertExpectations(t)
	}
}

func TestItem_Sync_BadJSON(t *testing.T) {
	router, cfg, _, _ := newItemTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/items/sync", strings.NewReader(`{`))
	req.Header.Set("Content-Type", "application/json")
	addItemAuthCookie(t, req, 5, cfg.AuthSecret)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
