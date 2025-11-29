package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Тест: без заголовка Accept-Encoding: gzip — ответа без сжатия
func TestWithGzip_NoAcceptEncoding(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	})
	h := WithGzip(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status want 200, got %d", rr.Code)
	}
	if ce := rr.Header().Get("Content-Encoding"); ce != "" {
		t.Fatalf("unexpected Content-Encoding: %q", ce)
	}
	if string(rr.Body.Bytes()) != "hello" {
		t.Fatalf("unexpected body: %q", rr.Body.String())
	}
}

// Тест: с Accept-Encoding: gzip — ответ сжат и корректно распаковывается
func TestWithGzip_WithAcceptEncoding(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// намеренно ставим Content-Length, чтобы убедиться, что мидлварь его убирает
		w.Header().Set("Content-Length", "5")
		_, _ = w.Write([]byte("hello"))
	})
	h := WithGzip(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected gzip Content-Encoding, got %q", rr.Header().Get("Content-Encoding"))
	}

	// распаковываем
	gr, err := gzip.NewReader(bytes.NewReader(rr.Body.Bytes()))
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	defer gr.Close()
	data, err := io.ReadAll(gr)
	if err != nil {
		t.Fatalf("failed to read gzipped body: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected ungzipped body: %q", string(data))
	}
}
