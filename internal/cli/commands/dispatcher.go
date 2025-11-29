package commands

import (
	"GophKeeper/internal/config"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
)

// Dispatch is the single entry point to execute CLI commands.
// It prints help and usage messages and returns a process exit code.
func Dispatch(ctx context.Context, cfg *config.Config, args []string) int {
	// If user passed global --help after flags parsing, show global usage
	for _, a := range os.Args[1:] {
		if a == "--help" || a == "-h" {
			fmt.Fprint(Out, FormatGlobalUsage())
			return 0
		}
	}

	if !flag.Parsed() {
		flag.Parse()
	}

	if len(args) == 0 {
		fmt.Fprint(Out, FormatGlobalUsage())
		return 2
	}

	name := strings.ToLower(args[0])
	if name == "help" { // gkcli help [command]
		if len(args) == 1 {
			fmt.Fprint(Out, FormatGlobalUsage())
			return 0
		}
		if c, ok := Get(args[1]); ok {
			fmt.Fprintf(Out, "Usage: %s\n", c.Usage())
			return 0
		}
		fmt.Fprintf(Out, "Unknown command: %s\n\n", args[1])
		fmt.Fprint(Out, FormatGlobalUsage())
		return 2
	}

	c, ok := Get(name)
	if !ok {
		fmt.Fprintf(Out, "Unknown command: %s\n\n", name)
		fmt.Fprint(Out, FormatGlobalUsage())
		return 2
	}

	err := c.Run(ctx, cfg, args[1:])
	switch err {
	case nil:
		return 0
	case ErrUsage:
		fmt.Fprintf(Out, "Usage: %s\n", c.Usage())
		return 2
	default:
		fmt.Fprintf(Out, "%s error: %v\n", name, err)
		return 1
	}
}
