package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

// Дымовой тест: проверяем, что мидлварь логирования не паникует и корректно проксирует ответ
func TestWithLogging_Smoke(t *testing.T) {
	SetLogger(zap.NewNop().Sugar())

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot) // 418
		_, _ = w.Write([]byte("hello"))
	})

	h := WithLogging(next)

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusTeapot {
		t.Fatalf("status passthrough failed: got %d", rr.Code)
	}
	if rr.Body.String() != "hello" {
		t.Fatalf("body passthrough failed: %q", rr.Body.String())
	}
}
