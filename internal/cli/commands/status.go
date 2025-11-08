package commands

import (
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

func Status(baseURL string) error {
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
