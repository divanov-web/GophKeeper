package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"GophKeeper/internal/cli/commands"
	"GophKeeper/internal/config"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	// Load unified config (env + flags)
	cfg := config.NewConfig()

	if cfg.Version {
		printVersion()
		return
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// dispatcher
	exitCode := commands.Dispatch(ctx, cfg, flag.Args())
	if exitCode == 0 {
		return
	}
	os.Exit(exitCode)
}

func printVersion() {
	fmt.Printf("GophKeeper CLI\nVersion: %s\nBuild date: %s\n", version, buildDate)
}
