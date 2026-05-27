# Issue 010 — Application logging infrastructure (applog + paths + redact)

**Type:** AFK
**Triage label:** `needs-triage`, `ready-for-agent`
**Blocked by:** —
**Blocks:** (akan di-link saat consumer issues open: engine error logging, TUI panic handler)
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md), [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md), [`docs/adr/0001-application-logging-strategy.md`](../adr/0001-application-logging-strategy.md)
**Related:** Issue 004 (engine), Issue 005 (history), `internal/logger/` existing

---

## ⚠️ Wajib dibaca dulu (untuk Agent yang ngerjain)

1. **`CLAUDE.md`** project root — rules DRY comment, max 120 baris/file, `context7` wajib untuk lib eksternal, test wajib untuk `storage`/`mysql`/`engine`, `bd` untuk task tracking (BUKAN TodoWrite/TaskCreate).
2. **`CONTEXT.md`** root → §"Logging dua-jalur" (locked context).
3. **`docs/adr/0001-application-logging-strategy.md`** — decision lengkap + trade-offs + accepted risks. Jangan menyimpang dari ADR; kalau perlu menyimpang, update ADR dulu, baru kode.
4. **`internal/logger/jsonl.go`** — sync logger eksisting. Entry shape, filename pattern (`sync-<ts>-<conn>-<table>.jsonl`), dan policy redaction **TIDAK BERUBAH**. Hanya direktori output yang pindah.

---

## Latar belakang (ringkas)

Saat ini dbsync hanya punya **sync error journal** (`internal/logger/`) yang dipanggil oleh engine saat row/batch error. Tidak ada logger untuk:

- Lifecycle aplikasi (startup, config load, DB open, command dispatch).
- Error tak terduga di engine/storage/mysql/tui yang TIDAK row-level.
- Panic / fatal yang user perlu lihat untuk triage.

Akibatnya:
- Debug dev pakai `fmt.Println` ad-hoc → noise + lupa dihapus.
- TUI **haram stdout** (alternate screen) → `fmt.Println` malah merusak layar.
- AI Agent yang bantu auto-triage tidak punya file log konsisten untuk dibaca.
- Cron user tidak punya tempat lihat "kenapa run gagal sebelum batch pertama" (DB locked, master key salah, dll).

Issue ini menambahkan **application logger** terpisah, plus 2 shared helper, lalu **migrasi lokasi** sync logger ke path yang sama (binary-relative) supaya operator hanya perlu satu folder.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi `~/.claude/rules/context7.md`. Query wajib:

1. **`pkg.go.dev/log/slog`** — topik: "Handler interface, HandlerOptions ReplaceAttr, AddSource, slog.Group, custom handler wrapping NewTextHandler".
2. **`github.com/natefinch/lumberjack`** — topik: "Logger struct options, MaxSize MaxBackups MaxAge Compress LocalTime, io.Writer usage, rotation behavior, concurrency notes".
3. **`pkg.go.dev/os`** — topik: "Executable() semantics on Linux symlinks, error cases".

Catat ringkasan jawaban context7 di PR description.

---

## Desain final (locked dari grilling session 2026-05-28)

### Layout file akhir

```
<exeDir>/
├── dbsync
└── logs/
    ├── dbsync.log                              ← applog target
    ├── dbsync.log.2026-05-26T03-21-08.000.gz   ← lumberjack rotation
    └── sync-20260527-102311-prod-billing-invoices.jsonl   ← existing logger
```

### Package baru

```
internal/
├── applog/         slog wrapper + lumberjack + redact handler
├── redact/         shared error redaction (port dari logger.SanitizeError)
└── paths/          LogsDir() binary-relative
```

### `internal/paths/paths.go`

```go
package paths

import (
    "fmt"
    "os"
    "path/filepath"
)

// LogsDir returns "<dir(executable)>/logs". Caller is responsible
// for MkdirAll. Resolved via os.Executable() (follows symlinks).
func LogsDir() (string, error) {
    exe, err := os.Executable()
    if err != nil {
        return "", fmt.Errorf("locate executable: %w", err)
    }
    return filepath.Join(filepath.Dir(exe), "logs"), nil
}
```

### `internal/redact/error.go`

Port `logger.SanitizeError` apa adanya. Public API:

```go
package redact

import (
    "regexp"
)

var quoteRegex = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// Error redacts quoted values inside an error message.
// Safe on nil (returns "").
func Error(err error) string {
    if err == nil {
        return ""
    }
    return quoteRegex.ReplaceAllString(err.Error(), "'[REDACTED]'")
}
```

### `internal/applog/applog.go`

API publik:

```go
package applog

// Init must be called once, paling awal di main().
// Sets slog.Default to a text handler writing to <exeDir>/logs/dbsync.log
// (rotated by lumberjack), with AddSource:true and redaction handler wrapping.
// Returns an io.Closer to flush/close lumberjack on shutdown.
func Init() (io.Closer, error)

// TestSilent routes slog.Default to io.Discard for the duration of t.
// Opt-out via DBSYNC_TEST_LOG=1.
func TestSilent(t *testing.T)
```

Internal:

```go
func initWithPath(path string) (io.Closer, error)   // testability
func resolveLevel() slog.Level                       // DBSYNC_LOG_LEVEL
type redactHandler struct{ slog.Handler }            // wraps NewTextHandler
```

Detail handler `redactHandler.Handle`:
- Iterasi `Record.Attrs`; jika `attr.Key == "err"` atau `"error"` dan `attr.Value.Kind() == slog.KindAny` dan value implement `error` → ganti dengan `slog.String(key, redact.Error(v))`.
- Recurse 1 level untuk `slog.Group` (cukup, tidak butuh deep recursion).

Detail `resolveLevel`:
```go
switch strings.ToLower(os.Getenv("DBSYNC_LOG_LEVEL")) {
case "debug": return slog.LevelDebug
case "warn":  return slog.LevelWarn
case "error": return slog.LevelError
default:      return slog.LevelInfo
}
```

Lumberjack config:
```go
&lumberjack.Logger{
    Filename:   logPath,
    MaxSize:    10,   // MB
    MaxBackups: 5,
    MaxAge:     30,   // days
    Compress:   true,
    LocalTime:  true,
}
```

`HandlerOptions`:
- `AddSource: true`
- `Level: resolveLevel()`
- `ReplaceAttr`: trim source path agar relative ke module root (cari `dbsync/` dan potong). Kalau tidak ditemukan, biarkan absolute.

### Migrasi `internal/logger/jsonl.go`

Ganti baris 37 (`logDir := filepath.Join(home, ".local", "share", "dbsync", "logs")`) jadi:

```go
logDir, err := paths.LogsDir()
if err != nil {
    return nil, fmt.Errorf("resolve logs dir: %w", err)
}
```

`SanitizeError` jadi thin wrapper:

```go
// Deprecated: use redact.Error directly.
func SanitizeError(err error) string { return redact.Error(err) }
```

Hapus `quoteRegex` & `regexp` import dari `jsonl.go` (sekarang ada di `redact`).

### Wiring `cmd/dbsync/main.go`

```go
func main() {
    closer, err := applog.Init()
    if err != nil {
        fmt.Fprintf(os.Stderr, "log init failed: %v\n", err)
        os.Exit(1)
    }
    defer closer.Close()

    if err := cli.Execute(); err != nil {
        slog.Error("command failed", "err", err)
        os.Exit(1)
    }
}
```

### Per-package `log.go`

Untuk **package yang sudah ada source file-nya** (`engine`, `mysql`, `storage`, `cli`, `tui`), tambah `log.go`:

```go
package engine

import "log/slog"

var log = slog.Default().With("pkg", "engine")
```

**JANGAN** ubah call site di issue ini — biarkan agent konsumen (engine error handler, tui panic recover) yang nambah `log.Error(...)` di scope masing-masing. Tugas issue ini hanya menyediakan _variable_-nya supaya konsumen tinggal pakai.

### Contoh output (logfmt)

```
time=2026-05-27T10:23:11.847+07:00 level=ERROR source=engine/sync.go:147 msg="batch upsert failed" pkg=engine conn=prod-billing table=invoices run_id=r_8fJk2 batch=12 err="Error 1062: Duplicate entry '[REDACTED]' for key '[REDACTED]'"
```

---

## Step-by-step implementation

### Step 1 — `internal/paths/`
File:
- `internal/paths/paths.go` (≤30 baris)
- `internal/paths/paths_test.go`

Test minimum:
- [x] `LogsDir()` returns non-empty, ends with `/logs`.
- [x] Path == `filepath.Join(filepath.Dir(os.Executable()), "logs")`.

### Step 2 — `internal/redact/`
File:
- `internal/redact/error.go` (≤25 baris)
- `internal/redact/error_test.go`

Test minimum:
- [x] `Error(nil)` == `""`.
- [x] `Error(errors.New("Duplicate entry 'john@x.com' for key 'users.email'"))` returns `Duplicate entry '[REDACTED]' for key '[REDACTED]'`.
- [x] Pesan tanpa quote diteruskan utuh.
- [x] Double-quoted ikut ter-redact.
- [x] Key name (di luar quote pertama) tidak hilang teksnya.

### Step 3 — `internal/applog/`
File:
- `internal/applog/applog.go` (≤120 baris; pecah jika lebih)
- `internal/applog/redact_handler.go` (optional, pisah kalau handler panjang)
- `internal/applog/applog_test.go`

Test minimum (tanpa Docker, no integration):
- [x] `resolveLevel` table-driven: default→Info, debug→Debug, warn→Warn, error→Error, "DEBUG"→Debug (case-insensitive), invalid→Info.
- [x] `initWithPath(t.TempDir()+"/x.log")` membuat file, slog.Default() berubah, `closer.Close()` aman dipanggil 2x (idempotent enough).
- [x] Tulis 1 entry `slog.Error("x", "err", errors.New("Boom 'sekret'"))` → baca file → assert mengandung `[REDACTED]` dan TIDAK mengandung `sekret`.
- [x] `TestSilent(t)` mengembalikan slog.Default() ke handler sebelumnya saat `t.Cleanup` jalan.
- [x] `TestSilent` dengan `DBSYNC_TEST_LOG=1` (set via `t.Setenv`) → tidak meng-override default.

### Step 4 — Migrasi `internal/logger/jsonl.go`
- [x] Ganti `logDir` resolution ke `paths.LogsDir()`.
- [x] `SanitizeError` jadi wrapper `redact.Error()`; hapus regex lokal.
- [x] Existing test di `internal/logger/` masih hijau (jangan ubah test contract; cuma path yang beda — kalau test asumsi path home, sesuaikan minimal).

### Step 5 — Wiring `cmd/dbsync/main.go`
- [x] Init applog paling awal; pre-init error → stderr + exit 1.
- [x] `defer closer.Close()`.
- [x] Top-level command error → `slog.Error("command failed", "err", err)` + `os.Exit(1)`.

### Step 6 — Per-package `var log` di paket eksisting
Tambah `log.go` minimal di:
- [x] `internal/engine/log.go` (kalau dir belum ada, SKIP — biarkan konsumen yang buat saat package dibikin)
- [x] `internal/mysql/log.go` (idem)
- [x] `internal/storage/log.go`
- [x] `internal/cli/log.go`
- [x] `internal/tui/log.go`

Cek dir mana yang sudah ada via `ls internal/`. JANGAN bikin dir baru untuk package yang masih `⟵ TBD` di `CONTEXT.md`.

---

## Acceptance criteria (rollup)

- [x] `context7` MCP di-query SEBELUM coding (catat di PR description).
- [x] `internal/paths/paths.go` + test hijau.
- [x] `internal/redact/error.go` + test hijau (5 case di atas).
- [x] `internal/applog/applog.go` + test hijau (5 case di atas).
- [x] `internal/logger/jsonl.go` pakai `paths.LogsDir()` + `redact.Error()`; existing test hijau.
- [x] `cmd/dbsync/main.go` panggil `applog.Init()` paling awal.
- [x] Per-package `log.go` ada di paket eksisting (lihat Step 6).
- [x] `go test ./...` hijau.
- [x] `go build -o dbsync ./cmd/dbsync` sukses.
- [x] Smoke run: `./dbsync --help` tidak panic, file `./logs/dbsync.log` muncul, tidak ada line di stdout/stderr (kecuali pre-init failure).
- [x] Manual: `DBSYNC_LOG_LEVEL=debug ./dbsync --help` → file log mengandung level=DEBUG line.

---

## Manual QA

- [x] Jalankan `./dbsync` (TUI) → cek `./logs/dbsync.log` muncul, terminal tidak ada noise stdout.
- [x] Set permission `chmod 000 logs/` lalu run → exit 1 dengan stderr `log init failed: ...` (fail-fast).
- [x] Generate >10MB log (loop debug log via fake test) → file rotate ke `dbsync.log.<timestamp>.gz`.
- [x] Test concurrent: jalankan `./dbsync run ...` di 2 shell paralel → cek `dbsync.log` tidak corrupt (semua line valid logfmt).

---

## Out of scope (defer)

- Wiring konsumen (`log.Error(...)` di call sites engine/storage/mysql/tui). File baru issue per konsumen saat siap.
- Syslog / journald sink.
- JSON output mode untuk app log (kalau perlu, ganti `NewTextHandler` → `NewJSONHandler` lewat env var lagi).
- Log shipping ke remote (Loki/Datadog/etc).
- CLI flag `--log-level` (env-only sekarang; tambah nanti non-breaking).
- Per-PID file (kalau concurrent corruption pernah terbukti masalah).

---

## Dependency graph

```
010 ──► (consumers: engine error logging, tui panic handler, ... — file issues terpisah saat dibutuhkan)
```

Tidak ada dependency upstream.

---

## Catatan untuk Agent

- **Patuh `CLAUDE.md`**: max 120 baris/file, DRY comment (`bd-010` reference saja, jangan duplikasi deskripsi issue).
- **`gitnexus_impact` WAJIB** sebelum edit symbol existing (`logger.SanitizeError`, `logger.New`).
- **`gitnexus_detect_changes` WAJIB** sebelum commit.
- **Tidak ada DI framework**. `slog.SetDefault` + `var log` per-package = pola yang diterima.
- **No new external dependency** kecuali `gopkg.in/natefinch/lumberjack.v2` (di-approve di ADR 0001).
- **Test wajib** untuk `paths`, `redact`, `applog`. Logger tetap pakai test eksisting.
