# dbsync v1 — Product Requirements Document

**Status:** Draft
**Date:** 2026-05-26
**Owner:** TBD
**Triage label:** `needs-triage`
**Related:** [`docs/ARCHITECTURE.md`](./ARCHITECTURE.md)

---

## Problem Statement

DBA dan developer yang mengelola dua MySQL host (mis. production → staging,
atau primary → reporting replica yang struktur tabelnya beda sedikit) saat
ini melakukan sync tabel secara ad-hoc: `mysqldump | mysql`, custom shell
script, atau klik manual di GUI tool. Pendekatan ini punya beberapa
masalah konkret:

1. **Tidak ada visibility riwayat sync.** "Tabel `users` terakhir di-sync
   kapan?" — tidak ada yang tahu kecuali ditulis manual di spreadsheet.
2. **Tidak idempoten / tidak resumable.** Kalau sync gagal di tengah
   (network blip, OOM kill), harus mulai dari nol — buang waktu untuk
   tabel jutaan row.
3. **Tidak ada column mapping.** Kalau tabel destination punya kolom
   tambahan (mis. `synced_at`, `tenant_id`) atau nama beda, harus tulis
   SQL custom per kasus.
4. **Credentials tersebar.** Password MySQL di-hardcode di shell script
   atau disimpan plaintext di config file — risiko leak tinggi.
5. **Tidak cocok untuk cron.** Kebanyakan tool butuh interaksi manual;
   tidak ada exit code yang bisa di-monitor.

User butuh satu tool yang bisa dipakai **interaktif** (saat eksplorasi /
setup) dan **non-interaktif** (saat dijalankan via cron untuk sync
periodik), dengan credentials yang aman dan riwayat yang tercatat.

## Solution

Satu binary Go (`dbsync`) dengan dua mode operasi yang berbagi storage
SQLite yang sama:

- **TUI mode** (`dbsync`) — bubbletea-based, untuk setup koneksi,
  konfigurasi column mapping, dan menjalankan sync secara interaktif
  dengan progress bar dan error panel.
- **CLI mode** (`dbsync run --connection=NAME --table=TBL`) — cron-friendly,
  exit code semantik, output ringkas ke stdout.

Sync bersifat **satu arah** (source → dest), per-tabel, dengan strategi
**upsert** (`INSERT ... ON DUPLICATE KEY UPDATE`) sehingga aman untuk
di-resume dari checkpoint. Credentials MySQL **selalu** dienkripsi di
SQLite dengan AES-256-GCM; kunci enkripsi diturunkan dari master password
(prompt TUI) atau env var `DBSYNC_MASTER_KEY` (cron).

Semua run tercatat di tabel `sync_history` (audit trail) dan checkpoint
aktif disimpan di `sync_checkpoints` untuk recovery.

## User Stories

### Setup & Connection Management

1. As a DBA, I want to add a MySQL connection pair (source + destination)
   via TUI form, so that saya tidak perlu edit config file manual.
2. As a DBA, I want connection password tersimpan terenkripsi di SQLite,
   so that database file aman kalau ke-leak ke backup atau git.
3. As a DBA, I want test connection ke source dan dest sebelum disimpan,
   so that typo host/credential ketahuan saat itu juga, bukan saat sync.
4. As a DBA, I want melihat daftar koneksi yang sudah disimpan dengan
   alias name, so that saya bisa identify koneksi tanpa lihat host
   detail.
5. As a DBA, I want edit dan hapus koneksi existing dari TUI, so that
   saya bisa rotasi credential atau pindah server tanpa edit SQL manual.
6. As a security-conscious operator, I want master password di-prompt
   sekali per TUI session dan tidak pernah ditulis ke disk, so that
   memory dump tidak bocorin key permanen.

### Table Mapping

7. As a DBA, I want browse tabel yang ada di source database, so that
   saya bisa pilih tabel mana yang mau di-sync tanpa hafal nama.
8. As a DBA, I want mapping kolom source → dest 1:1 default (kolom
   nama sama auto-mapped), so that saya tidak perlu setup ulang untuk
   kasus umum.
9. As a DBA, I want bisa override nama kolom dest (rename), so that
   saya bisa sync tabel `user` (source) ke `users` dengan kolom
   `email_address` (source) → `email` (dest).
10. As a DBA, I want set default value untuk kolom dest yang tidak ada
    di source (mis. `synced_at = NOW()`, `tenant_id = 42`), so that
    schema dest yang punya kolom tambahan tetap bisa di-fill.
11. As a DBA, I want lihat warning kalau ada kolom di dest yang
    NOT NULL tapi tidak ada mapping-nya, so that saya tidak diserprise
    sync gagal di tengah karena constraint violation.

### Sync Execution

12. As a DBA, I want jalankan sync per-tabel dari TUI dengan progress
    bar dan ETA, so that saya tahu sync masih jalan atau hang.
13. As a DBA, I want lihat error panel real-time saat sync, so that
    saya bisa intervene cepat (cancel, fix data, retry) tanpa nunggu
    sampai selesai.
14. As an operator, I want jalankan sync via CLI dengan flag
    `--connection=X --table=Y`, so that bisa di-trigger dari cron tanpa
    interaksi.
15. As an operator, I want CLI return exit code 0 (success), 1 (partial
    fail), 2 (fatal), so that cron job monitoring bisa alerting yang
    benar.
16. As an operator, I want option `--all-tables` untuk sync semua tabel
    yang sudah di-mapping pada connection itu, so that nightly batch
    sync cukup 1 command.
17. As a DBA, I want sync yang interrupted bisa di-resume dari checkpoint
    terakhir, so that sync tabel 10 juta row tidak harus dari awal kalau
    network drop di tengah.
18. As a DBA, I want sync idempotent — kalau saya run ulang sync yang
    sudah selesai, tidak ada duplicate atau error, so that saya bisa
    aman trigger sync berkali-kali tanpa takut data rusak.
19. As an operator, I want CLI `--dry-run` flag yang preview berapa row
    yang akan di-sync tanpa eksekusi upsert, so that saya bisa estimasi
    durasi run produksi.

### Visibility & Audit

20. As a DBA, I want lihat history sync (kapan, durasi, row count,
    success/fail) per tabel dari TUI, so that saya bisa jawab "kapan
    terakhir tabel X di-sync".
21. As a DBA, I want error log per sync run di-tulis ke file JSON lines,
    so that bisa di-grep, di-parse oleh log aggregator, atau dibaca
    junior tanpa harus buka TUI.
22. As a security-conscious operator, I want log file TIDAK menyimpan
    nilai row plaintext (hanya `?` placeholder + PK), so that PII tidak
    bocor lewat log.
23. As a DBA, I want lihat checkpoint aktif (tabel yang lagi running
    atau interrupted) dari TUI, so that saya bisa decide resume atau
    reset.

### CI / Cron Integration

24. As an operator, I want set env var `DBSYNC_MASTER_KEY` untuk supply
    encryption key non-interaktif, so that cron job tidak nge-hang nunggu
    prompt password.
25. As an operator, I want CLI fail fast dengan pesan jelas kalau
    `DBSYNC_MASTER_KEY` tidak set dan stdin non-interactive, so that
    saya tidak salah konfigurasi cron lalu silent fail.
26. As an operator, I want binary yang cross-compile gampang (no CGo),
    so that saya bisa build di laptop Mac → deploy ke Linux server
    tanpa Docker.

## Implementation Decisions

### Architecture Overview

Bottom-up modular implementation, frontend-agnostic core. Engine
mengemit events lewat channel; TUI dan CLI sama-sama consume channel
yang sama. Storage SQLite jadi **single source of truth** — TUI tidak
boleh pegang state config in-memory yang divergen dari DB.

### Modules

| Module | Responsibility | Test priority |
|---|---|---|
| `internal/crypto` | AES-256-GCM encrypt/decrypt, scrypt KDF. Pure, no I/O. | High |
| `internal/config` | Master password lifecycle: env var → stdin prompt → salt file load. Isolasi dari `crypto` agar `crypto` tetap pure. | Medium |
| `internal/storage` | SQLite repo: connections, mappings, checkpoints, history. Migration idempotent. | High |
| `internal/logger` | JSON-lines log writer untuk error per-row + per-batch. Redaction (no raw values). | Medium |
| `internal/mysql` | Pool (10 max open, 5 idle, 5min lifetime, 30s timeout), `DetectPK` via INFO_SCHEMA, `DescribeColumns`, `SelectBatch`, `Upsert`. | High |
| `internal/engine` | `Run(ctx, opts) (<-chan Event, error)` orchestrator. Frontend-agnostic. | High |
| `internal/cli` | cobra flag parsing, wire engine + logger + stdout. Exit code 0/1/2. | Low |
| `internal/tui` | bubbletea models (conn list, table picker, mapping form, run screen). Subscribe ke engine channel via tea.Cmd polling. | Low (manual QA) |

### Module Interfaces (high-level)

- **`crypto`** — `Encrypt(plaintext, key) (b64string, error)`,
  `Decrypt(b64string, key) (plaintext, error)`,
  `DeriveKey(masterPassword, salt) (key, error)`. Format ciphertext:
  `base64(nonce || ciphertext)`.
- **`config`** — `LoadMasterKey(ctx) ([]byte, error)`. Internal logic:
  cek `DBSYNC_MASTER_KEY` env var (must be 32-byte hex) → fallback
  ke stdin prompt (kalau interactive) → fail dengan instruction kalau
  non-interactive dan env tidak set. Salt file di
  `~/.config/dbsync/salt` (auto-generate kalau belum ada).
- **`storage`** — satu file per tabel (connections.go, mappings.go,
  checkpoints.go, history.go) dengan repo struct pegang `*sql.DB`.
  CRUD method standar (Insert/Update/Delete/Get/List). Migration
  dijalankan di `db.go` saat Open.
- **`logger`** — `New(path string) (*Logger, error)`, method
  `RowError(batch, pk, err, sqlTemplate)`, `BatchError(...)`. Path
  pattern: `~/.local/share/dbsync/logs/sync-{ts}-{conn}-{table}.jsonl`.
  Field minimal: `{timestamp, level, batch, row_pk, error, sql_template}`.
- **`mysql`** — `Pool` wrapper struct, `DetectPK(db, schema, table) ([]string, error)`,
  `DescribeColumns(db, schema, table) ([]Column, error)`,
  `SelectBatch(db, table, pkCols, lastPK, limit) ([]Row, error)`,
  `Upsert(tx, table, mapping, rows) (n int, error)`.
- **`engine`** — `Run(ctx, opts EngineOpts) (<-chan Event, error)`.
  Event types: `ProgressEvent`, `BatchErrorEvent`, `RowErrorEvent`,
  `DoneEvent`. `EngineOpts` carry connection ID, table name, dry-run
  flag, batch size (default 1000).

### Data Flow & Algorithms

- **Sync algorithm:** batch loop dengan ukuran 1000 row,
  `SELECT * FROM source WHERE pk > last_pk ORDER BY pk LIMIT 1000`,
  transaksi per batch di dest dengan `SET FOREIGN_KEY_CHECKS=0`,
  upsert via `INSERT ... ON DUPLICATE KEY UPDATE` dengan mapping +
  default resolution.
- **Mapping resolution** (3 cases di `sync_column_mappings`):
  1. `source_column NOT NULL, default_value NULL` → ambil dari source
  2. `source_column NULL, default_value NOT NULL` → tulis literal
     default (mis. `NOW()`, `42`)
  3. Both NOT NULL → fallback: source kalau NOT NULL, default kalau
     source NULL
- **PK detection:** query
  `INFORMATION_SCHEMA.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA=? AND TABLE_NAME=? AND CONSTRAINT_NAME='PRIMARY'`.
  Support composite PK.
- **Resume:** kalau ada row di `sync_checkpoints` untuk (connection,
  table) dengan status `running` atau `interrupted`, prompt user
  (TUI) atau auto-resume (CLI default) dari `last_pk_value`. Idempotency
  dijamin oleh upsert.
- **Error policy:** fail-fast per batch. Row error → ROLLBACK batch,
  log ke JSONL + emit `BatchErrorEvent`, abort run (no retry di v1).

### Schema (already in `001_init.sql`)

4 tabel: `connections`, `sync_column_mappings`, `sync_checkpoints`,
`sync_history`. Constraint penting: `UNIQUE (connection_id, table_name)`
di `sync_checkpoints` (1 checkpoint aktif per tabel), CHECK constraint
pada `status` field, FK CASCADE dari `connections` ke semua child table.
Trigger auto-bump `updated_at` di `connections` dan `sync_checkpoints`.

### Security

- AES-256-GCM dengan nonce per-row (random 12 byte, prepended).
- scrypt KDF: N=32768, r=8, p=1, keyLen=32. Salt random per-install,
  disimpan di `~/.config/dbsync/salt`.
- Key derivation hanya saat first decrypt per session; hasil cached
  in-memory (`[]byte`), tidak ditulis ke disk.
- `DBSYNC_MASTER_KEY` env var: must be 32-byte hex (64 char). Validasi
  panjang sebelum dipakai.
- Log redaction: SQL template di-log (`INSERT INTO users (a, b) VALUES (?, ?)`),
  argumen tidak. PK di-log karena dibutuhkan untuk debugging tapi
  asumsinya PK non-sensitive (auto-increment ID).
- Connection string di error message: password field selalu `***`.

### CLI Contract

```
dbsync                                            # TUI mode
dbsync run --connection=NAME --table=TBL          # single sync
dbsync run --connection=NAME --all-tables         # all mapped tables
dbsync run --connection=NAME --table=TBL --dry-run
dbsync version
dbsync help
```

Exit codes: `0` success, `1` partial fail (some rows/tables failed),
`2` fatal (connection error, master key error, schema mismatch).

### Dependencies (locked)

- Go 1.22+
- `github.com/charmbracelet/bubbletea` + `lipgloss` + `bubbles`
- `github.com/spf13/cobra`
- `github.com/go-sql-driver/mysql`
- `modernc.org/sqlite` (pure-Go, no CGo)
- `golang.org/x/crypto/scrypt`
- `golang.org/x/term`

## Testing Decisions

### What makes a good test

- **Test external behavior, not implementation details.** Test bahwa
  `Encrypt(x); Decrypt(ciphertext) == x`, bukan bahwa Encrypt memanggil
  `cipher.NewGCM` internal.
- **Tests harus deterministic.** No real network, no real time-dependent
  assertion. Pakai `:memory:` SQLite untuk storage tests.
- **Tests harus runnable di laptop developer tanpa setup eksternal.**
  Untuk MySQL adapter, prefer testcontainers atau skip-on-missing-env
  pattern (`if os.Getenv("MYSQL_TEST_DSN") == "" { t.Skip() }`).
- **Table-driven test** untuk fungsi dengan banyak kasus (mapping
  resolver, KDF input validation).

### Modules to test (per scope decision)

| Module | Test approach |
|---|---|
| `crypto` | Round-trip encrypt/decrypt, KDF determinism (same input → same key), tamper detection (modify ciphertext → Decrypt errors), nonce uniqueness (encrypt 1000x sama input → 1000 unique ciphertext). |
| `storage` | `:memory:` SQLite + run migration. Test CRUD per repo, FK CASCADE behavior (delete connection → mappings/checkpoints/history terhapus), UNIQUE constraint, migration idempotent (run 2x tidak error). |
| `mysql` | Testcontainers `mysql:8`. Test `DetectPK` (single + composite PK), `DescribeColumns`, `SelectBatch` pagination correctness, `Upsert` idempotency. Skip kalau Docker tidak available. |
| `engine` | Integration test pakai `:memory:` storage + testcontainers MySQL. Test mapping resolution (3 cases), resume from checkpoint (sync sebagian, kill, resume, verify final row count), dry-run mode. |

### Prior art

Tidak ada test existing di repo (proyek baru). Pattern referensi yang
akan dipakai:

- **stdlib `database/sql` tests** untuk pola SQL repo testing dengan
  `:memory:` SQLite.
- **`testcontainers-go` examples** untuk MySQL container lifecycle
  (`mysql.Run(ctx, "mysql:8")`).
- **`go-cmp`** untuk struct diff assertion.

## Out of Scope

Eksplisit tidak dikerjakan di v1 (akan jadi PRD terpisah kalau perlu):

- **Bi-directional sync / conflict resolution.** v1 satu arah saja.
- **Multi-database engine** (Postgres, SQL Server). MySQL only.
- **Built-in scheduler.** Pakai cron OS. Tidak ada daemon mode.
- **Web UI / metrics dashboard.** TUI + CLI saja.
- **Schema migration.** Asumsi struktur tabel source dan dest sudah
  cocok (atau mapping yang handle perbedaan kolom). dbsync tidak
  generate `ALTER TABLE`.
- **Schema diff preview tool.** v2.
- **Retry policy / exponential backoff.** v1 fail-fast.
- **Multi-table parallel sync.** Sequential per tabel di v1.
- **Webhook / Slack notification.** v2.
- **Encryption-at-rest untuk SQLite file itu sendiri.** Hanya field
  password yang dienkripsi; SQLite file diasumsikan aman karena di
  user's local machine. Kalau dibutuhkan, user bisa pakai LUKS / FileVault.
- **Hot reload config.** Restart untuk apply perubahan koneksi.

## Further Notes

- **Performance baseline (informational, bukan target keras):** dengan
  batch 1000 row dan jaringan LAN, ekspektasi ~50k-100k row/menit untuk
  tabel dengan ~20 kolom. Akan diverifikasi setelah engine selesai.
- **Backwards compatibility:** v1 adalah baseline; tidak ada constraint
  backward compat. Schema migration `001_init.sql` akan jadi base untuk
  semua migration berikutnya — jangan edit setelah release.
- **Local development:** `go build -o dbsync ./cmd/dbsync` + jalankan
  dengan MySQL lokal (`docker run mysql:8`). Test database scaffold akan
  disediakan di test setup helper.
- **Documentation deliverable:** README user-facing + `docs/ARCHITECTURE.md`
  technical reference. PRD ini bisa di-archive setelah v1 ship.
- **Estimated effort:** scaffold sudah selesai (Phase 1).
  Phase 2 (crypto + config) ~1 hari, Phase 3 (storage + logger) ~1-2 hari,
  Phase 4 (mysql + engine) ~3-4 hari, Phase 5 (CLI) ~1 hari,
  Phase 6 (TUI) ~3-5 hari. Total v1: ~2 minggu solo dev.

---

*End of PRD.*
