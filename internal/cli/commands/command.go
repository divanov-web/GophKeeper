package commands

import (
	"GophKeeper/internal/config"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// ErrUsage is returned by a command when arguments are invalid and usage should be shown.
var ErrUsage = errors.New("usage")

// Command represents a CLI subcommand.
type Command interface {
	// Name returns the command name as typed by the user, e.g. "login".
	Name() string
	// Description is a short human-readable description shown in help.
	Description() string
	// Usage returns the exact usage string, e.g. "login <login> <password>".
	Usage() string
	// Run executes the command with provided args (without the command name).
	Run(ctx context.Context, cfg *config.Config, args []string) error
}

// registry holds available commands by name.
var registry = map[string]Command{}

// Out — общий writer для вывода CLI. По умолчанию os.Stdout, но в тестах может переназначаться.
var Out io.Writer = os.Stdout

// RegisterCmd adds a command to the registry. Should be called from init() of each command.
func RegisterCmd(cmd Command) {
	registry[cmd.Name()] = cmd
}

// Get returns a command by name.
func Get(name string) (Command, bool) {
	c, ok := registry[name]
	return c, ok
}

// List returns all registered commands sorted by name.
func List() []Command {
	list := make([]Command, 0, len(registry))
	for _, c := range registry {
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Name() < list[j].Name() })
	return list
}

// FormatGlobalUsage builds a help text for all commands.
func FormatGlobalUsage() string {
	lines := []string{
		"GophKeeper CLI",
		"",
		"Usage:",
		"  gkcli [--base-url <host:port>|URL] <command> [args]",
		"",
		"Commands:",
	}
	for _, c := range List() {
		lines = append(lines, fmt.Sprintf("  %-28s %s", c.Usage(), c.Description()))
	}
	return strings.Join(lines, "\n") + "\n"
}
