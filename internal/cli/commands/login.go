package commands

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"GophKeeper/internal/cli/api"
)

type LoginRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func Login(baseURL, login, password string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/user/login"
	req := LoginRequest{Login: login, Password: password}
	resp, body, err := api.PostJSON(endpoint, req, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		if err := api.PersistAuthFromResponse(resp); err != nil {
			return fmt.Errorf("saving auth: %w", err)
		}
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid login or password")
	}
	return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
}
