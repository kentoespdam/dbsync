# dbsync

One-way MySQL table sync tool with a TUI + CLI dual interface.

> **Status:** v1 — scaffolding phase. Not functional yet.

## Quick Start

```bash
# Build
go build -o dbsync ./cmd/dbsync

# Launch TUI
./dbsync

# Run a single sync (cron-friendly)
./dbsync run --connection=prod-to-staging --table=users
```

## Design

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for the full design:
layering, data flow, SQLite schema, security model, and module
responsibilities.

## Layout

```
cmd/dbsync/         entry point (TUI vs CLI dispatch)
internal/tui/       bubbletea models, views, update
internal/cli/       CLI command handlers (cobra)
internal/engine/    sync orchestrator (pure logic)
internal/storage/   SQLite repo (single source of truth)
internal/mysql/     MySQL adapter (pools, schema, ops)
internal/crypto/    AES-256-GCM + scrypt KDF
```

## Stack

- Go 1.22+
- TUI: [bubbletea](https://github.com/charmbracelet/bubbletea) + [lipgloss](https://github.com/charmbracelet/lipgloss)
- CLI: [cobra](https://github.com/spf13/cobra)
- MySQL: [go-sql-driver/mysql](https://github.com/go-sql-driver/mysql)
- SQLite: [`modernc.org/sqlite`](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGo)
- Crypto: `golang.org/x/crypto/scrypt`

## License

TBD.
