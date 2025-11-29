package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Доп.кейс: PostJSON без токена — Cookie заголовок не должен устанавливаться
func TestPostJSON_NoToken_NoCookieHeader(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := r.Header.Get("Cookie"); c != "" {
			t.Fatalf("Cookie must be empty when token not provided, got: %q", c)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	resp, body, err := PostJSON(ts.URL, map[string]any{"x": 1}, "")
	if err != nil {
		t.Fatalf("PostJSON err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	_ = body
}

// Доп.кейсы PersistAuthFromResponse: auth_token вторым cookie и пустое значение
func TestPersistAuthFromResponse_MultipleCookies_And_EmptyValueError(t *testing.T) {
	// auth_token вторым — должен сохраниться
	{
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "other", Value: "abc"}).String())
		resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "auth_token", Value: "tok-2"}).String())
		if err := PersistAuthFromResponse(resp); err != nil {
			t.Fatalf("persist second cookie: %v", err)
		}
	}
	// auth_token присутствует, но пустой — ошибка
	{
		resp := &http.Response{Header: http.Header{}}
		resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "auth_token", Value: ""}).String())
		if err := PersistAuthFromResponse(resp); err == nil {
			t.Fatalf("expected error for empty auth_token cookie value")
		}
	}
}

// Доп.кейс: PostJSON — сетевая ошибка (недостижимый адрес)
func TestPostJSON_NetworkError(t *testing.T) {
	if _, _, err := PostJSON("http://127.0.0.1:1", map[string]any{"a": 1}, ""); err == nil {
		t.Fatalf("expected network error for unreachable URL")
	}
}

// Доп.кейс: PostJSON — ошибка при создании запроса (невалидный URL)
func TestPostJSON_InvalidURL_NewRequestError(t *testing.T) {
	if _, _, err := PostJSON("http://[::1", map[string]any{"a": 1}, ""); err == nil {
		t.Fatalf("expected new request error for invalid URL")
	}
}

// Доп.кейс: PostMultipartBlob — ошибка при создании запроса (невалидный URL)
func TestPostMultipartBlob_InvalidURL_NewRequestError(t *testing.T) {
	if _, _, err := PostMultipartBlob("http://[::1", "id", []byte{1}, []byte{1}, ""); err == nil {
		t.Fatalf("expected new request error for invalid URL")
	}
}
