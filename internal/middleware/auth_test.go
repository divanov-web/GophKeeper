package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Тест: SetLoginCookie + WithAuth — user_id попадает в контекст
func TestWithAuth_ValidCookieSetsUserID(t *testing.T) {
	const secret = "test-secret"

	// next-хендлер читает user_id из контекста
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid, ok := GetUserIDFromContext(r.Context()); ok {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("uid:"))
			_, _ = w.Write([]byte(string(rune(uid)))) // не важно содержимое; важен код 200
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	})

	h := WithAuth(secret)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rrCookie := httptest.NewRecorder()
	_ = SetLoginCookie(rrCookie, 77, secret)
	for _, c := range rrCookie.Result().Cookies() {
		req.AddCookie(c)
	}

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 with valid cookie, got %d", rr.Code)
	}
}

// Тест: отсутствие cookie — user_id не устанавливается
func TestWithAuth_NoCookieLeavesAnonymous(t *testing.T) {
	h := WithAuth("any-secret")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetUserIDFromContext(r.Context()); ok {
			t.Fatalf("user id must not be set without cookie")
		}
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

// Тест: невалидный токен — user_id не устанавливается
func TestWithAuth_InvalidToken(t *testing.T) {
	// Сгенерируем cookie с секретом A, а проверять будем секретом B
	rrCookie := httptest.NewRecorder()
	_ = SetLoginCookie(rrCookie, 5, "secret-A")

	h := WithAuth("secret-B")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := GetUserIDFromContext(r.Context()); ok {
			t.Fatalf("user id must not be set with invalid token")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range rrCookie.Result().Cookies() {
		req.AddCookie(c)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}
