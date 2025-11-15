package main

import (
	"flag"
	"fmt"
	"os"

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

	// dispatcher
	exitCode := commands.Dispatch(cfg, flag.Args())
	if exitCode == 0 {
		return
	}
	os.Exit(exitCode)
}

func printVersion() {
	fmt.Printf("GophKeeper CLI\nVersion: %s\nBuild date: %s\n", version, buildDate)
}
