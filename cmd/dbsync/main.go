// Package main is the dbsync entry point.
//
// dbsync dispatches between two modes:
//
//   - TUI mode  : when invoked with no subcommand (`dbsync`)
//   - CLI mode  : when invoked with a subcommand (`dbsync run ...`)
//
// Both modes share the same SQLite storage as the single source of truth.
// See docs/ARCHITECTURE.md for the full design.
package main

import (
	"fmt"
	"os"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v1.0.0-dev"

func main() {
	// No args  -> TUI mode
	// Has args -> CLI mode (run, list, etc.)
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	switch os.Args[1] {
	case "version", "--version", "-v":
		fmt.Printf("dbsync %s\n", version)
	case "run":
		runCLI(os.Args[2:])
	case "help", "--help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", os.Args[1])
		printHelp()
		os.Exit(2)
	}
}

func runTUI() {
	// TODO: wire up bubbletea program here (internal/tui)
	fmt.Println("dbsync TUI — coming soon")
	fmt.Println("(stub: internal/tui not yet implemented)")
}

func runCLI(args []string) {
	// TODO: wire up cobra / flag parser here (internal/cli)
	fmt.Printf("dbsync CLI run — args=%v\n", args)
	fmt.Println("(stub: internal/cli not yet implemented)")
}

func printHelp() {
	fmt.Print(`dbsync — MySQL table sync tool

Usage:
  dbsync                       Launch interactive TUI
  dbsync run --connection=NAME --table=TBL
                               Run a single sync (CLI / cron mode)
  dbsync run --connection=NAME --all-tables
                               Sync every mapped table in the connection
  dbsync version               Print version
  dbsync help                  Show this help

Docs:
  docs/ARCHITECTURE.md
`)
}
