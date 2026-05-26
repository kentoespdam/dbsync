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

	"github.com/user/dbsync/internal/cli"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v1.0.0-dev"

func main() {
	// If version is requested specifically
	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "--version" || os.Args[1] == "-v") {
		fmt.Printf("dbsync %s\n", version)
		return
	}

	// No args -> TUI mode
	if len(os.Args) < 2 {
		runTUI()
		return
	}

	// Execute CLI (Cobra)
	cli.Execute()
}

func runTUI() {
	// TODO: wire up bubbletea program here (internal/tui)
	fmt.Println("dbsync TUI — coming soon")
	fmt.Println("(stub: internal/tui not yet implemented)")
	fmt.Println("Run 'dbsync help' for CLI usage.")
}
