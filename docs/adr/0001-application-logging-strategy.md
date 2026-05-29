# ADR 0001 — Application Logging Strategy

**Status:** Accepted (superseded in part by [ADR 0003](./0003-remove-mysql-error-redaction.md))
**Date:** 2026-05-28
**Decision drivers:** debug ergonomics dev & production, log dapat dipakai AI Agent untuk auto-perbaikan, cron-friendly, TUI tidak boleh nulis ke stdout.

> **Note 2026-05-29:** Section "Decision" sub-3 (`internal/redact/`) dan komponen `redactHandler` di sub-1 di-supersede oleh ADR 0003. Error MySQL kini ditulis apa adanya. Password redaction di `internal/mysql/pool.go` tidak terpengaruh.

---

## Context

`dbsync` punya dua kebutuhan log yang **berbeda audiens & tujuan**:

1. **App lifecycle / debug / error review** — startup, config load, DB open, command dispatch, panic, error tak terduga di engine/storage/mysql/tui. Konsumennya manusia dan AI Agent yang melakukan triage. Belum ada.
2. **Per-sync error journal** — row/batch error saat sync jalan; sudah diimplementasikan di `internal/logger/` sebagai JSONL ber-stempel `sync-<ts>-<conn>-<table>.jsonl`. Sudah dipakai oleh engine.

Dua kebutuhan ini punya **bentuk dan cara konsumsi yang beda**:
- App log: continuous text stream, butuh rotation karena hidup terus.
- Sync log: discrete file per-run, retention manual, format machine-readable per-entry untuk replay/diff.

Mencampurnya jadi satu jalur akan memaksa kompromi (format vs filename vs rotation policy).

Selain itu, TUI (bubbletea) **tidak boleh menulis ke stdout/stderr** karena layar dikelola alternate screen — log harus file-only.

Plaintext = SSoT principle juga berlaku: log harus berada di **lokasi yang predictable per-deployment**, bukan tersebar di home dir. Operator yang taruh binary di `/opt/dbsync/` ingin log di `/opt/dbsync/logs/`, bukan `~/.local/share/dbsync/logs/` user yang men-trigger cron.

---

## Decision

Pisah jadi **dua package log + dua helper shared**:

### 1. `internal/applog/` (BARU)
Application logger umum, bungkus `log/slog` stdlib + `gopkg.in/natefinch/lumberjack.v2` untuk rotation.

- Output: `<exeDir>/logs/dbsync.log`, format text (logfmt: `key=value`), `AddSource:true`, level via env `DBSYNC_LOG_LEVEL` (debug|info|warn|error; default info).
- Rotation defaults: `MaxSize=10MB`, `MaxBackups=5`, `MaxAge=30d`, `Compress=true`, `LocalTime=true`.
- File-only writer (tidak nulis ke stdout/stderr — TUI safety).
- `func Init() (io.Closer, error)` dipanggil paling awal di `cmd/dbsync/main.go`. Gagal init → `fmt.Fprintf(os.Stderr, ...)` + `os.Exit(1)`. Tidak ada CLI flag.
- `slog.SetDefault(...)` di-set sekali; per-package pakai `var log = slog.Default().With("pkg", "<name>")`.
- Custom handler intercept atribut bernama `err`/`error` lewat `redact.Error()` (recurse 1 level untuk `slog.Group`).
- `func TestSilent(t *testing.T)` arahkan slog ke `io.Discard`; opt-in real log via `DBSYNC_TEST_LOG=1`.

### 2. `internal/logger/` (KEEP)
Sync error journal yang sudah ada. **Entry shape, filename pattern, dan policy redaction tetap.** Hanya direktori output yang dipindah dari `~/.local/share/dbsync/logs/` ke `<exeDir>/logs/` via `paths.LogsDir()`.

### 3. `internal/redact/` (BARU)
Helper netral untuk strip quoted values di error MySQL/SQL.
- `func Error(err error) string` — port dari `logger.SanitizeError` apa adanya (regex `'[^']*'|"[^"]*"` → `'[REDACTED]'`).
- Dipakai oleh `applog` (lewat handler) DAN `logger` (per-entry).

### 4. `internal/paths/` (BARU)
- `func LogsDir() (string, error)` — `filepath.Join(filepath.Dir(os.Executable()), "logs")`.
- Centralized agar TUI dan CLI tidak divergen.

### Layout file akhir

```
<exeDir>/
├── dbsync
└── logs/
    ├── dbsync.log
    ├── dbsync.log.2026-05-26T03-21-08.000.gz
    └── sync-20260527-102311-prod-billing-invoices.jsonl
```

### Contoh entry app log

```
time=2026-05-27T10:23:11.847+07:00 level=ERROR source=engine/sync.go:147 msg="batch upsert failed" pkg=engine conn=prod-billing table=invoices run_id=r_8fJk2 batch=12 err="Error 1062: Duplicate entry '[REDACTED]' for key '[REDACTED]'"
```

---

## Alternatives considered

1. **Satu logger untuk semua (slog single).** Ditolak: bentuk sync-error-journal (discrete file per run, replay-able JSONL) tidak cocok dengan slog continuous stream + rotation. Mencampurnya akan bikin format kompromi.
2. **Pakai `zap` / `zerolog` / `logrus`.** Ditolak: prinsip "Simple > clever, no DI" + slog stdlib sudah cukup, dan lebih familiar untuk junior dev + local AI.
3. **Log di `~/.local/share/dbsync/logs/` (mengikuti XDG).** Ditolak oleh user: operator butuh log per-deployment (per binary path), terutama untuk cron context yang `HOME` bisa beda dengan user yang setup.
4. **Tulis ke stdout + `tee` di shell.** Tidak applicable: TUI memakai alternate screen, stdout = corruption. CLI bisa, tapi konsistensi dengan TUI menang.
5. **Inline redaction di setiap callsite.** Ditolak: error-prone, ada DRY violation. Centralized `redact` lebih aman.

---

## Trade-offs / Accepted risks

1. **Concurrent write ke `dbsync.log` dari 2+ proses (cron + TUI simultan).** Lumberjack thread-safe **dalam satu proses**, tidak inter-process. Risiko:
   - Pada Linux, `write()` ≤ `PIPE_BUF` (4KB) atomik untuk regular file di banyak FS umum (ext4 default). Logfmt line dbsync jauh di bawah batas itu, jadi interleaving partial-line sangat jarang.
   - Race saat rotation (dua proses rename file bersamaan) bisa kehilangan beberapa baris. Frekuensi rotate (10MB tiap rotasi) untuk pola pakai dbsync sangat rendah.
   - **Diterima** karena dbsync bukan high-throughput logger, dan dampaknya cuma baris ter-interleave (masih valid logfmt per-line). Jika di kemudian hari terbukti masalah, ganti ke per-PID file (`dbsync-<pid>.log`) atau syslog tanpa breaking change ke konsumen.

2. **`os.Executable()` resolusi via symlink.** `filepath.Dir(os.Executable())` mengembalikan path symlink-resolved di Linux (sesuai docs). Jika operator pakai symlink `/usr/local/bin/dbsync → /opt/dbsync/bin/dbsync`, log akan ditulis ke `/opt/dbsync/bin/logs/`. Diterima: itu lokasi binary real, predictable.

3. **Tidak ada CLI flag `--log-level`.** Env-only (`DBSYNC_LOG_LEVEL`). Diterima: konsisten dengan `DBSYNC_MASTER_KEY` pattern; cron environment lebih natural pakai env vars. Bisa ditambah nanti tanpa breaking change.

4. **`AddSource:true` ada cost performance ~5-10%.** Diterima untuk dev ergonomy — `file:line` adalah anchor utama untuk AI Agent auto-debugging.

5. **Test default = silent.** Resiko: bug di logger init bisa lolos test. Mitigasi: `applog_test.go` punya unit test eksplisit yang **tidak** pakai `TestSilent`; opt-in `DBSYNC_TEST_LOG=1` untuk lihat output di test lain saat perlu.

---

## Consequences

**Positif:**
- Debug ergonomi naik signifikan; AI Agent punya file `dbsync.log` yang predictable untuk auto-triage.
- Sync error journal tetap clean dan compatible dengan integration test yang sudah ada.
- Redaction terpusat → konsisten antar dua jalur log.

**Negatif:**
- Tambah 3 package internal (`applog`, `redact`, `paths`) — total baris tipis tapi node count naik.
- Operator harus tahu log ada di binary-relative path; di-cover di README dan ADR ini.

**Migrasi:**
- `internal/logger/jsonl.go` baris 37 ganti ke `paths.LogsDir()`.
- `logger.SanitizeError` jadi thin wrapper ke `redact.Error()` (atau callsite langsung pakai `redact.Error()`).
- Semua package konsumen (engine, mysql, storage, cli, tui) tambah file `log.go` dengan `var log = slog.Default().With("pkg", "<name>")`.

---

## References

- `CONTEXT.md` → §"Logging dua-jalur"
- `docs/issues/010-application-logging.md` → execution plan
- `internal/logger/jsonl.go` → existing sync logger (unchanged shape)
- [`gopkg.in/natefinch/lumberjack.v2`](https://github.com/natefinch/lumberjack)
- Go stdlib `log/slog` docs
