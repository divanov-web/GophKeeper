package commands

import (
	"GophKeeper/internal/config"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"GophKeeper/internal/cli/api"
	"GophKeeper/internal/cli/auth"
)

type dataResponse struct {
	Result string `json:"result"`
}

type statusCmd struct{}

func (statusCmd) Name() string        { return "status" }
func (statusCmd) Description() string { return "Check auth status (calls /api/user/test)" }
func (statusCmd) Usage() string       { return "status" }

func (statusCmd) Run(cfg *config.Config, args []string) error {
	if len(args) != 0 {
		return ErrUsage
	}
	baseURL := cfg.ServerURL
	endpoint := strings.TrimRight(baseURL, "/") + "/api/user/test"
	token, _ := auth.LoadToken()
	resp, body, err := api.PostJSON(endpoint, struct{}{}, token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var dr dataResponse
	if err := json.Unmarshal(body, &dr); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	fmt.Println("Status:", dr.Result)
	return nil
}

func init() { RegisterCmd(statusCmd{}) }
