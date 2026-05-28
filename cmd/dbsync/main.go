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
	"log/slog"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kentoespdam/dbsync/internal/applog"
	"github.com/kentoespdam/dbsync/internal/cli"
	"github.com/kentoespdam/dbsync/internal/config"
	"github.com/kentoespdam/dbsync/internal/storage"
	"github.com/kentoespdam/dbsync/internal/tui"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "v1.0.0-dev"

func main() {
	closer, err := applog.Init()
	if err != nil {
		fmt.Fprintf(os.Stderr, "log init failed: %v\n", err)
		os.Exit(1)
	}
	defer closer.Close()

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
	if err := cli.Execute(); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}

func runTUI() {
	if wiped, err := config.EnsureConsistentState(); err != nil {
		fmt.Fprintf(os.Stderr, "Error reconciling portable state: %v\n", err)
		os.Exit(1)
	} else if wiped {
		fmt.Fprintln(os.Stderr, "Notice: salt/db pair was inconsistent; wiped orphan so first-run setup can proceed.")
	}

	dbPath, err := config.DBPath()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error determining database path: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0700); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating data directory: %v\n", err)
		os.Exit(1)
	}

	db, err := storage.Open(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	p := tea.NewProgram(tui.New(db), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
