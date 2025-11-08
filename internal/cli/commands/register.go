package commands

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"GophKeeper/internal/cli/api"
)

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func Register(baseURL, login, password string) error {
	endpoint := strings.TrimRight(baseURL, "/") + "/api/user/register"
	req := RegisterRequest{Login: login, Password: password}
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
	if resp.StatusCode == http.StatusConflict {
		return errors.New("login already in use")
	}
	return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
}
