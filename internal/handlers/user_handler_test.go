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
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Minimal mocks
type mockUserRepo struct{ mock.Mock }

func (m *mockUserRepo) CreateUser(ctx context.Context, user *model.User) (*model.User, error) {
	args := m.Called(ctx, user)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *mockUserRepo) GetUserByLogin(ctx context.Context, login string) (*model.User, error) {
	args := m.Called(ctx, login)
	if u, ok := args.Get(0).(*model.User); ok {
		return u, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.UserRepository = (*mockUserRepo)(nil)

type mockItemRepo struct{ mock.Mock }

func (m *mockItemRepo) GetItemsUpdatedSince(ctx context.Context, userID int64, since time.Time) ([]model.Item, error) {
	args := m.Called(ctx, userID, since)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) GetByID(ctx context.Context, userID int64, id string) (*model.Item, error) {
	args := m.Called(ctx, userID, id)
	if v, ok := args.Get(0).(*model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}
func (m *mockItemRepo) Create(ctx context.Context, it *model.Item) error {
	args := m.Called(ctx, it)
	return args.Error(0)
}
func (m *mockItemRepo) UpdateWithVersion(ctx context.Context, userID int64, id string, expectedVersion int64, updates map[string]any) (int64, error) {
	args := m.Called(ctx, userID, id, expectedVersion, updates)
	return args.Get(0).(int64), args.Error(1)
}
func (m *mockItemRepo) ListAll(ctx context.Context, userID int64) ([]model.Item, error) {
	args := m.Called(ctx, userID)
	if v, ok := args.Get(0).([]model.Item); ok {
		return v, args.Error(1)
	}
	return nil, args.Error(1)
}

var _ repo.ItemRepository = (*mockItemRepo)(nil)

type mockBlobRepo struct{ mock.Mock }

func (m *mockBlobRepo) CreateIfAbsent(ctx context.Context, id string, cipher, nonce []byte) (bool, error) {
	args := m.Called(ctx, id, cipher, nonce)
	return args.Bool(0), args.Error(1)
}

var _ repo.BlobRepository = (*mockBlobRepo)(nil)

// --- Helpers ---
func newTestRouter(t *testing.T, ur repo.UserRepository) http.Handler {
	t.Helper()
	cfg := &config.Config{AuthSecret: "test-secret", BlobMaxSizeMB: 1}
	logger := zap.NewNop().Sugar()

	userSvc := service.NewUserService(ur)
	// для user‑тестов item‑сервисы не используются, дадим заглушки
	itemSvc := service.NewItemService(&mockItemRepo{}, &mockBlobRepo{}, logger)

	h := handlers.NewHandler(userSvc, itemSvc, logger, cfg)
	return h.Router
}

func addAuthCookie(t *testing.T, req *http.Request, userID int64, secret string) {
	t.Helper()
	rr := httptest.NewRecorder()
	_ = middleware.SetLoginCookie(rr, userID, secret)
	for _, c := range rr.Result().Cookies() {
		req.AddCookie(c)
	}
}

// --- Tests ---
func TestUser_Register(t *testing.T) {
	m := new(mockUserRepo)
	router := newTestRouter(t, m)

	t.Run("ok", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "john").Return((*model.User)(nil), nil).Once()
		created := &model.User{ID: 42, Login: "john"}
		m.On("CreateUser", mock.Anything, mock.MatchedBy(func(u *model.User) bool { return u.Login == "john" && u.Password != "" })).Return(created, nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"login":"john","password":"p"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		hasCookie := false
		for _, c := range rr.Result().Cookies() {
			if c.Name == "auth_token" {
				hasCookie = true
			}
		}
		assert.True(t, hasCookie, "Set-Cookie auth_token expected")
		m.AssertExpectations(t)
	})

	t.Run("conflict", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "john").Return(&model.User{ID: 1, Login: "john"}, nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"login":"john","password":"p"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusConflict, rr.Code)
		m.AssertExpectations(t)
	})
}

func TestUser_Login(t *testing.T) {
	m := new(mockUserRepo)
	router := newTestRouter(t, m)

	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.DefaultCost)

	t.Run("ok", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "alice").Return(&model.User{ID: 2, Login: "alice", Password: string(hash)}, nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"login":"alice","password":"secret"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		hasCookie := false
		for _, c := range rr.Result().Cookies() {
			if c.Name == "auth_token" {
				hasCookie = true
			}
		}
		assert.True(t, hasCookie)
		m.AssertExpectations(t)
	})

	t.Run("unauthorized", func(t *testing.T) {
		m.ExpectedCalls = nil
		m.On("GetUserByLogin", mock.Anything, "alice").Return(&model.User{ID: 2, Login: "alice", Password: string(hash)}, nil).Once()

		req := httptest.NewRequest(http.MethodPost, "/api/user/login", strings.NewReader(`{"login":"alice","password":"bad"}`))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusUnauthorized, rr.Code)
		m.AssertExpectations(t)
	})
}

func TestUser_Status(t *testing.T) {
	m := new(mockUserRepo)
	router := newTestRouter(t, m)

	t.Run("anonymous", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/user/test", nil)
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var body struct {
			Result string `json:"result"`
		}
		_ = json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&body)
		assert.Equal(t, "anonymous", body.Result)
	})

	t.Run("authorized", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/user/test", nil)
		addAuthCookie(t, req, 77, "test-secret")
		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		var body struct {
			Result string `json:"result"`
		}
		_ = json.NewDecoder(bytes.NewReader(rr.Body.Bytes())).Decode(&body)
		assert.Contains(t, body.Result, "User ID = 77")
	})
}
