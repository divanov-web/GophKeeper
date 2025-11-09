package commands

import (
	"GophKeeper/internal/config"
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

type registerCmd struct{}

func (registerCmd) Name() string        { return "register" }
func (registerCmd) Description() string { return "Register a new user" }
func (registerCmd) Usage() string       { return "register <login> <password>" }

func (registerCmd) Run(cfg *config.Config, args []string) error {
	if len(args) < 2 {
		return ErrUsage
	}
	login := args[0]
	password := args[1]
	baseURL := cfg.ServerURL
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
		fmt.Println("Registered successfully")
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return errors.New("login already in use")
	}
	return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
}

func init() { RegisterCmd(registerCmd{}) }
