package api

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"

	fsrepo "GophKeeper/internal/cli/repo/fs"
)

// PostJSON sends a JSON POST request. If token is non-empty, it is passed as auth cookie.
func PostJSON(url string, payload any, token string) (*http.Response, []byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, nil, err
	}
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Cookie", "auth_token="+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	return resp, body, nil
}

// PersistAuthFromResponse извлекает auth cookie из ответа и сохраняет его через файловое хранилище.
func PersistAuthFromResponse(resp *http.Response) error {
	store := fsrepo.AuthFSStore{}
	for _, c := range resp.Cookies() {
		if c.Name == "auth_token" && c.Value != "" {
			return store.Save(c.Value)
		}
	}
	return fmt.Errorf("no auth cookie in response")
}

// PostMultipartBlob отправляет multipart/form-data запрос для загрузки блоба.
// Поля:
// - id: строковое поле
// - cipher: file-part с бинарным содержимым
// - nonce: строковое поле (base64)
func PostMultipartBlob(url, id string, cipher, nonce []byte, token string) (*http.Response, []byte, error) {
	if id == "" {
		return nil, nil, fmt.Errorf("empty id")
	}
	if len(cipher) == 0 || len(nonce) == 0 {
		return nil, nil, fmt.Errorf("empty cipher/nonce")
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	// id
	if err := mw.WriteField("id", id); err != nil {
		return nil, nil, err
	}
	// nonce как base64 строка
	if err := mw.WriteField("nonce", base64.StdEncoding.EncodeToString(nonce)); err != nil {
		return nil, nil, err
	}
	// cipher как файл
	cf, err := mw.CreateFormFile("cipher", "cipher.bin")
	if err != nil {
		return nil, nil, err
	}
	if _, err := cf.Write(cipher); err != nil {
		return nil, nil, err
	}
	_ = mw.Close()

	req, err := http.NewRequest(http.MethodPost, url, &buf)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Cookie", "auth_token="+token)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	body, _ := io.ReadAll(resp.Body)
	// гарантируем закрытие body
	_ = resp.Body.Close()
	// Нормализуем переносы строк в теле для сообщений
	if len(body) > 0 {
		body = []byte(strings.TrimSpace(string(body)))
	}
	return resp, body, nil
}
