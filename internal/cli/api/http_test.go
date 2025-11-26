package api

import (
	fsrepo "GophKeeper/internal/cli/repo/fs"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// helper: перенастройка конфиг‑каталога в temp
func setTempCfg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", dir)
	} else {
		t.Setenv("XDG_CONFIG_HOME", dir)
	}
	// и для совместимости с путями клиента
	t.Setenv("CLIENT_DB_PATH", filepath.Join(dir, "db"))
	_ = os.MkdirAll(filepath.Join(dir, "db"), 0o700)
	return dir
}

func TestPostJSON_SendsToken_And_ParsesBody(t *testing.T) {
	setTempCfg(t)
	// test server проверяет cookie и JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c := r.Header.Get("Cookie"); !strings.Contains(c, "auth_token=tok123") {
			t.Fatalf("Cookie header missing token, got: %q", c)
		}
		var m map[string]any
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			t.Fatalf("bad json: %v", err)
		}
		if m["x"] != float64(1) { // JSON number → float64
			t.Fatalf("unexpected payload: %#v", m)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	resp, body, err := PostJSON(ts.URL+"/api", map[string]any{"x": 1}, "tok123")
	if err != nil {
		t.Fatalf("PostJSON err: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	if strings.TrimSpace(string(body)) != `{"ok":true}` {
		t.Fatalf("body: %s", string(body))
	}
}

func TestPostJSON_JSONMarshalError(t *testing.T) {
	// chan в payload вызовет ошибку json.Marshal
	_, _, err := PostJSON("http://example.invalid", map[string]any{"c": make(chan int)}, "")
	if err == nil {
		t.Fatalf("expected marshal error")
	}
}

func TestPersistAuthFromResponse_SaveAndNoCookie(t *testing.T) {
	setTempCfg(t)
	// success: есть Set-Cookie с auth_token
	{
		resp := &http.Response{Header: http.Header{}}
		// Добавим Set-Cookie вручную (http.SetCookie ожидает ResponseWriter)
		resp.Header.Add("Set-Cookie", (&http.Cookie{Name: "auth_token", Value: "tok-abc"}).String())
		if err := PersistAuthFromResponse(resp); err != nil {
			t.Fatalf("persist: %v", err)
		}
		// проверим, что токен читается из FS
		tok, err := (fsrepo.AuthFSStore{}).Load()
		if err != nil || tok != "tok-abc" {
			t.Fatalf("token not saved, got %q err=%v", tok, err)
		}
	}
	// error: нет cookie
	{
		resp := &http.Response{Header: http.Header{}}
		if err := PersistAuthFromResponse(resp); err == nil {
			t.Fatalf("expected error when no auth cookie")
		}
	}
}

func TestPostMultipartBlob_Success_201_And_200(t *testing.T) {
	setTempCfg(t)
	phase := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data;") {
			t.Fatalf("not multipart: %s", r.Header.Get("Content-Type"))
		}
		if !strings.Contains(r.Header.Get("Cookie"), "auth_token=tok") {
			t.Fatalf("missing auth cookie")
		}
		if err := r.ParseMultipartForm(10 << 20); err != nil {
			t.Fatalf("parse form: %v", err)
		}
		if r.FormValue("id") != "B1" {
			t.Fatalf("id mismatch: %s", r.FormValue("id"))
		}
		// nonce — строковое поле base64 из функции
		if r.FormValue("nonce") == "" {
			t.Fatalf("nonce missing")
		}
		// сначала 201, затем 200
		if phase == 0 {
			phase = 1
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(" {\n  \t\rcreated:true } ")) // лишние пробелы/переводы
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{\n\"created\":false}\n"))
	}))
	defer ts.Close()

	// Created: сервер отдаёт тело с ведущими/замыкающими пробелами/переводами строк —
	// функция должна триммировать края, но внутренние переводы строк допустимы.
	resp, body, err := PostMultipartBlob(ts.URL, "B1", []byte{1, 2}, []byte{9, 9, 9}, "tok")
	if err != nil {
		t.Fatalf("post mp 201: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status want 201, got %d", resp.StatusCode)
	}
	sb := string(body)
	if strings.HasPrefix(sb, " ") || strings.HasPrefix(sb, "\n") || strings.HasSuffix(sb, " ") || strings.HasSuffix(sb, "\n") || strings.HasSuffix(sb, "\r") || strings.HasSuffix(sb, "\t") {
		t.Fatalf("body should have no leading/trailing whitespace: %q", sb)
	}
	// OK
	resp, body, err = PostMultipartBlob(ts.URL, "B1", []byte{1}, []byte{2}, "tok")
	if err != nil {
		t.Fatalf("post mp 200: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status want 200, got %d", resp.StatusCode)
	}
	_ = body
}

func TestPostMultipartBlob_ValidationAndNetworkErrors(t *testing.T) {
	setTempCfg(t)
	// validation
	if _, _, err := PostMultipartBlob("http://example.invalid", "", []byte{1}, []byte{1}, ""); err == nil {
		t.Fatalf("empty id should fail")
	}
	if _, _, err := PostMultipartBlob("http://example.invalid", "B", nil, []byte{1}, ""); err == nil {
		t.Fatalf("empty cipher should fail")
	}
	if _, _, err := PostMultipartBlob("http://example.invalid", "B", []byte{1}, nil, ""); err == nil {
		t.Fatalf("empty nonce should fail")
	}
	// network error (заблокированный порт/невалидный URL)
	if _, _, err := PostMultipartBlob("http://127.0.0.1:1", "B", []byte{1}, []byte{1}, ""); err == nil {
		t.Fatalf("expected network error")
	}
}
