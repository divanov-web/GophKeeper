package commands

import (
	"GophKeeper/internal/config"
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

type loginCmd struct{}

func (loginCmd) Name() string        { return "login" }
func (loginCmd) Description() string { return "Login and store auth cookie" }
func (loginCmd) Usage() string       { return "login <login> <password>" }

func (loginCmd) Run(cfg *config.Config, args []string) error {
	if len(args) < 2 {
		return ErrUsage
	}
	login := args[0]
	password := args[1]
	baseURL := cfg.ServerURL
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
		fmt.Println("Logged in successfully")
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return errors.New("invalid login or password")
	}
	return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
}

func init() { RegisterCmd(loginCmd{}) }
