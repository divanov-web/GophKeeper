package commands

import (
	"GophKeeper/internal/config"
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"GophKeeper/internal/cli/api"
	fsrepo "GophKeeper/internal/cli/repo/fs"
	reposqlite "GophKeeper/internal/cli/repo/sqlite"
)

type RegisterRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type registerCmd struct{}

func (registerCmd) Name() string        { return "register" }
func (registerCmd) Description() string { return "Register a new user" }
func (registerCmd) Usage() string       { return "register <login> <password>" }

func (registerCmd) Run(ctx context.Context, cfg *config.Config, args []string) error {
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
		// remember last successful login
		if err := (fsrepo.AuthFSStore{}).SaveLogin(login); err != nil {
			return fmt.Errorf("save last login: %w", err)
		}
		// prepare per-user DB and run migrations
		st, _, err := reposqlite.OpenForUser(login)
		if err != nil {
			return fmt.Errorf("open user db: %w", err)
		}
		defer st.Close()
		if err := st.Migrate(); err != nil {
			return fmt.Errorf("migrate user db: %w", err)
		}
		fmt.Fprintln(Out, "Registered successfully")
		return nil
	}
	if resp.StatusCode == http.StatusConflict {
		return errors.New("login already in use")
	}
	return fmt.Errorf("server error: %s", strings.TrimSpace(string(body)))
}

func init() { RegisterCmd(registerCmd{}) }
