package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"GophKeeper/internal/cli/commands"
)

var (
	version   = "dev"
	buildDate = "unknown"
)

func main() {
	// Global flags
	baseURL := flag.String("base-url", "http://localhost:8081", "Base URL of the GophKeeper server")
	showVersion := flag.Bool("version", false, "Show client version and exit")
	flag.Parse()

	if *showVersion {
		printVersion()
		return
	}

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	cmd := strings.ToLower(args[0])
	switch cmd {
	case "register":
		if len(args) < 3 {
			fmt.Println("Usage: register <login> <password>")
			os.Exit(2)
		}
		login := args[1]
		password := args[2]
		if err := commands.Register(*baseURL, login, password); err != nil {
			fmt.Println("Register error:", err)
			os.Exit(1)
		}
		fmt.Println("Registered successfully")

	case "login":
		if len(args) < 3 {
			fmt.Println("Usage: login <login> <password>")
			os.Exit(2)
		}
		login := args[1]
		password := args[2]
		if err := commands.Login(*baseURL, login, password); err != nil {
			fmt.Println("Login error:", err)
			os.Exit(1)
		}
		fmt.Println("Logged in successfully")

	case "status":
		if err := commands.Status(*baseURL); err != nil {
			fmt.Println("Status error:", err)
			os.Exit(1)
		}

	case "version":
		printVersion()

	default:
		printUsage()
		os.Exit(2)
	}
}

func printUsage() {
	fmt.Println("GophKeeper CLI")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  gkcli [--base-url <url>] <command> [args]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  register <login> <password>  Register a new user")
	fmt.Println("  login <login> <password>     Login and store auth cookie")
	fmt.Println("  status                        Check auth status (calls /api/user/test)")
	fmt.Println("  version                       Show client version")
}

func printVersion() {
	fmt.Printf("GophKeeper CLI\nVersion: %s\nBuild date: %s\n", version, buildDate)
}
