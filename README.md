# dbsync

One-way MySQL table sync tool with a TUI + CLI dual interface.

> **Status:** v1.0.0-rc — Cross-platform binaries available.

## Installation

### Linux
1. Download the latest `.tar.gz` from [Releases](https://github.com/kentoespdam/dbsync/releases).
2. Extract: `tar -xzf dbsync_linux_amd64.tar.gz`.
3. Move to your path: `sudo mv dbsync /usr/local/bin/`.

### Windows
1. Download the latest `.zip` from [Releases](https://github.com/kentoespdam/dbsync/releases).
2. Extract the ZIP file.
3. Run `dbsync.exe` from PowerShell or Command Prompt.

## Windows Usage

Since the binary is not signed with a Microsoft-trusted certificate:
1. When you run `dbsync.exe` for the first time, Windows SmartScreen will show a "Windows protected your PC" warning.
2. Click **More info**.
3. Click **Run anyway**.

## Quick Start

```bash
# Launch TUI
dbsync

# Run a single sync (cron-friendly)
dbsync run --connection=prod-to-staging --table=users
```

## Releases

`dbsync` follows semantic versioning. Checksums for every release are available on the [Releases](https://github.com/kentoespdam/dbsync/releases) page for verification.

## Design
...
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
