package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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
