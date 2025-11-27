package commands

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"GophKeeper/internal/config"
)

// fakeCmd позволяет управлять возвратом ошибок из Run
type fakeCmd struct {
	name, usage, desc string
	run               func(ctx context.Context, cfg *config.Config, args []string) error
}

func (f fakeCmd) Name() string        { return f.name }
func (f fakeCmd) Description() string { return f.desc }
func (f fakeCmd) Usage() string       { return f.usage }
func (f fakeCmd) Run(ctx context.Context, cfg *config.Config, args []string) error {
	return f.run(ctx, cfg, args)
}

// перехват stdout на время теста
func withStdoutCapture(t *testing.T, fn func()) string {
	t.Helper()
	old := Out
	var buf bytes.Buffer
	Out = &buf
	defer func() { Out = old }()
	fn()
	return buf.String()
}

func TestDispatcher_HelpAndUnknown(t *testing.T) {
	// зарегистрированы login/register/status/items из init()
	out := withStdoutCapture(t, func() { _ = Dispatch(context.Background(), &config.Config{}, []string{}) })
	if !strings.Contains(out, "GophKeeper CLI") {
		t.Fatalf("global help expected")
	}

	out = withStdoutCapture(t, func() { _ = Dispatch(context.Background(), &config.Config{}, []string{"help"}) })
	if !strings.Contains(out, "Usage:") {
		t.Fatalf("usage expected")
	}

	code := Dispatch(context.Background(), &config.Config{}, []string{"help", "login"})
	if code != 0 {
		t.Fatalf("expected 0 for help login, got %d", code)
	}

	out = withStdoutCapture(t, func() { _ = Dispatch(context.Background(), &config.Config{}, []string{"help", "nope"}) })
	if !strings.Contains(out, "Unknown command") {
		t.Fatalf("unknown command message expected")
	}

	code = Dispatch(context.Background(), &config.Config{}, []string{"no-such"})
	if code != 2 {
		t.Fatalf("expected 2 for unknown command, got %d", code)
	}
}

func TestDispatcher_RunPaths(t *testing.T) {
	// зарегистрируем временную команду
	cmdOK := fakeCmd{name: "x", usage: "x", desc: "", run: func(_ context.Context, _ *config.Config, _ []string) error { return nil }}
	RegisterCmd(cmdOK)
	if code := Dispatch(context.Background(), &config.Config{}, []string{"x"}); code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	cmdUsage := fakeCmd{name: "u", usage: "u <arg>", desc: "", run: func(_ context.Context, _ *config.Config, _ []string) error { return ErrUsage }}
	RegisterCmd(cmdUsage)
	out := withStdoutCapture(t, func() { _ = Dispatch(context.Background(), &config.Config{}, []string{"u"}) })
	if !strings.Contains(out, "Usage: u <arg>") {
		t.Fatalf("usage text expected")
	}

	cmdErr := fakeCmd{name: "e", usage: "e", desc: "", run: func(_ context.Context, _ *config.Config, _ []string) error { return fmt.Errorf("boom") }}
	RegisterCmd(cmdErr)
	out = withStdoutCapture(t, func() { _ = Dispatch(context.Background(), &config.Config{}, []string{"e"}) })
	if !strings.Contains(out, "e error: boom") {
		t.Fatalf("error line expected, got: %s", out)
	}
}

func TestStatus_Run_Success_Errors_and_Usage(t *testing.T) {
	withTempConfig(t)
	// успех: 200 и корректный JSON
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/api/user/test") {
			t.Fatalf("path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"result":"authorized"}`))
	}))
	defer ts.Close()
	cfg := &config.Config{ServerURL: ts.URL}
	if err := (statusCmd{}).Run(context.Background(), cfg, []string{}); err != nil {
		t.Fatalf("status ok failed: %v", err)
	}

	// non-200
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer ts500.Close()
	if err := (statusCmd{}).Run(context.Background(), &config.Config{ServerURL: ts500.URL}, []string{}); err == nil {
		t.Fatalf("status should fail on non-200")
	}

	// битый JSON
	tsBad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{"))
	}))
	defer tsBad.Close()
	if err := (statusCmd{}).Run(context.Background(), &config.Config{ServerURL: tsBad.URL}, []string{}); err == nil {
		t.Fatalf("status must fail on bad json")
	}

	// ErrUsage при лишних аргументах
	if err := (statusCmd{}).Run(context.Background(), cfg, []string{"extra"}); err != ErrUsage {
		t.Fatalf("expected ErrUsage, got %v", err)
	}
}
