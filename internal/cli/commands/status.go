package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	fsrepo "GophKeeper/internal/cli/repo/fs"
	"GophKeeper/internal/config"
)

type dataResponse struct {
	Result string `json:"result"`
}

type statusCmd struct{}

func (statusCmd) Name() string        { return "status" }
func (statusCmd) Description() string { return "Check auth status (calls /api/user/test)" }
func (statusCmd) Usage() string       { return "status" }

func (statusCmd) Run(ctx context.Context, cfg *config.Config, args []string) error {
	if len(args) != 0 {
		return ErrUsage
	}
	baseURL := cfg.ServerURL
	endpoint := strings.TrimRight(baseURL, "/") + "/api/user/test"
	token, _ := (fsrepo.AuthFSStore{}).Load()
	// ограничим время выполнения запроса
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// Соберём JSON-тело запроса
	bodyBytes, _ := json.Marshal(struct{}{})
	req, err := http.NewRequestWithContext(tctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Cookie", "auth_token="+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Если это контекстная отмена/таймаут — вернём её явно
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var dr dataResponse
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &dr); err != nil {
		return fmt.Errorf("decode: %w", err)
	}
	fmt.Fprintln(Out, "Status:", dr.Result)
	return nil
}

func init() { RegisterCmd(statusCmd{}) }
