# dbsync — Architecture Design Document

**Version:** v1
**Date:** 2026-05-26
**Status:** Draft (post requirements gathering)

---

## 1. Tujuan & Scope

`dbsync` adalah aplikasi Go untuk sinkronisasi tabel MySQL satu arah (host A → host B).

**In scope (v1):**
- TUI interaktif untuk konfigurasi koneksi, mapping kolom, dan menjalankan sync
- CLI mode untuk automation (cron-friendly)
- Sync per-tabel dengan strategi upsert
- Resume dari checkpoint kalau interrupted
- Audit trail untuk visibility "tabel X terakhir sync kapan"

**Out of scope (v1):**
- Bi-directional sync / conflict resolution
- Multi-database engine (Postgres, dll.)
- Built-in scheduler (pakai cron OS)
- Web UI / metrics dashboard
- Schema migration (hanya data sync; struktur tabel diasumsikan sudah cocok)

**Design principle:** Simple > clever. Code harus bisa dibaca junior dev dan local AI model. Tidak ada DI framework, tidak ada abstraction berlapis.

---

## 2. High-Level Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Entry Points                            │
│  ┌──────────────────┐         ┌──────────────────────────┐  │
│  │  TUI (bubbletea) │         │  CLI (cobra/flag)        │  │
│  │  - Connections   │         │  dbsync run --conn=X     │  │
│  │  - Tables        │         │           --table=Y      │  │
│  │  - Mappings      │         │  dbsync run --conn=X     │  │
│  │  - Run / Monitor │         │           --all-tables   │  │
│  └────────┬─────────┘         └───────────┬──────────────┘  │
└───────────┼─────────────────────────────────┼───────────────┘
            │                                 │
            └──────────────┬──────────────────┘
                           ▼
            ┌──────────────────────────────┐
            │      Sync Engine (core)      │
            │  - Preflight (test, schema)  │
            │  - Mapping resolver          │
            │  - Batch loop + upsert       │
            │  - Checkpoint writer         │
            │  - Progress emitter          │
            └──┬───────────────────┬───────┘
               │                   │
               ▼                   ▼
   ┌────────────────────┐   ┌──────────────────────┐
   │   MySQL Adapter    │   │   Storage (SQLite)   │
   │  - Source pool     │   │  - connections       │
   │  - Dest pool       │   │  - column_mappings   │
   │  - INFO_SCHEMA     │   │  - checkpoints       │
   │  - Batch select    │   │  - history           │
   │  - Upsert exec     │   └──────────────────────┘
   └────────────────────┘            ▲
                                     │
                          ┌──────────┴──────────┐
                          │   Crypto module      │
                          │  AES-256-GCM         │
                          │  PBKDF2/scrypt KDF   │
                          │  master pwd / ENV    │
                          └──────────────────────┘
```

---

## 3. Project Layout

```
dbsync/
├── go.mod
├── go.sum
├── README.md
├── docs/
│   └── ARCHITECTURE.md         (file ini)
├── cmd/
│   └── dbsync/
│       └── main.go              (entry point: dispatch TUI vs CLI)
├── internal/
│   ├── tui/                     (bubbletea models, views, update)
│   │   ├── app.go               (root model, navigation)
│   │   ├── conn_list.go         (connection list/edit screen)
│   │   ├── table_picker.go      (table selection)
│   │   ├── mapping_form.go      (column mapping editor)
│   │   ├── run_screen.go        (progress + error panel)
│   │   └── styles.go            (lipgloss styles)
│   ├── cli/                     (CLI command handlers)
│   │   └── run.go
│   ├── engine/                  (sync engine — pure logic)
│   │   ├── engine.go            (Run() orchestrator)
│   │   ├── preflight.go         (test conn, schema validate)
│   │   ├── batch.go             (batch select + upsert)
│   │   ├── checkpoint.go        (resume logic)
│   │   └── progress.go          (metrics channel)
│   ├── storage/                 (SQLite repo — single source of truth)
│   │   ├── db.go                (open + migrate)
│   │   ├── connections.go
│   │   ├── mappings.go
│   │   ├── checkpoints.go
│   │   ├── history.go
│   │   └── migrations/
│   │       └── 001_init.sql
│   ├── mysql/                   (MySQL adapter)
│   │   ├── pool.go              (open with pool config)
│   │   ├── schema.go            (INFORMATION_SCHEMA queries)
│   │   └── ops.go               (select batch, upsert exec)
│   └── crypto/
│       └── crypto.go            (AES-GCM + KDF)
└── pkg/
    └── (nothing exported for v1 — semua internal)
```

**Konvensi:**
- `internal/` untuk semua kode app (tidak di-import luar)
- Tidak ada folder `pkg/` yang di-export di v1 — keep it simple
- Satu file = satu tanggung jawab; tidak boleh > ~300 lines per file

---

## 4. Data Flow — Sync Run

```
[User trigger: TUI "Run" / CLI run command]
            │
            ▼
1. Load connection from SQLite        ──> storage.connections
2. Decrypt password                   ──> crypto.Decrypt(masterKey)
3. Open MySQL pools (source + dest)   ──> mysql.pool
4. Preflight:
   a. SELECT 1 on both                ──> fail-fast
   b. Detect PK via INFO_SCHEMA       ──> mysql.schema
   c. Validate columns + types        ──> warning/error UI
5. Load column mappings               ──> storage.mappings
6. Check checkpoint exists?
   YES ──> resume from last_pk_value
   NO  ──> start fresh (last_pk_value=0)
7. Insert sync_history row (status=running, started_at=now)
8. SET FOREIGN_KEY_CHECKS=0 on dest
9. Batch loop:
   for each batch of 1000:
     a. SELECT * FROM source WHERE pk > last_pk LIMIT 1000
     b. BEGIN TX on dest
     c. INSERT ... ON DUPLICATE KEY UPDATE (with mapping + defaults)
     d. On row error:
          - ROLLBACK this batch only
          - Log to JSON file + TUI error panel
          - Stop processing (fail-fast per batch)
     e. On batch success:
          - COMMIT
          - Update checkpoint (last_pk_value, last_batch_completed)
          - Emit progress event
10. SET FOREIGN_KEY_CHECKS=1 on dest
11. Update sync_history (finished_at, status, row counts)
12. Close pools
```

**Resume semantics:** Karena upsert idempoten, restart dari checkpoint aman — batch yang sudah berhasil tidak akan duplicate, dan PK > last_pk_value memastikan no re-processing.

---

## 5. SQLite Schema (Final)

File migration: `internal/storage/migrations/001_init.sql`

```sql
-- ============================================================
-- 001_init.sql — dbsync v1 schema
-- ============================================================

PRAGMA foreign_keys = ON;

-- Koneksi MySQL (source + dest disimpan sebagai 1 row "pair")
CREATE TABLE IF NOT EXISTS connections (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL UNIQUE,           -- alias mis. "prod-to-staging"
    source_host     TEXT NOT NULL,
    source_port     INTEGER NOT NULL DEFAULT 3306,
    source_user     TEXT NOT NULL,
    source_password TEXT NOT NULL,                  -- AES-256-GCM encrypted (base64)
    source_db       TEXT NOT NULL,
    dest_host       TEXT NOT NULL,
    dest_port       INTEGER NOT NULL DEFAULT 3306,
    dest_user       TEXT NOT NULL,
    dest_password   TEXT NOT NULL,                  -- AES-256-GCM encrypted (base64)
    dest_db         TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Mapping kolom per (connection, table)
CREATE TABLE IF NOT EXISTS sync_column_mappings (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id   INTEGER NOT NULL,
    table_name      TEXT NOT NULL,
    source_column   TEXT,                           -- NULL kalau dest punya default value saja
    dest_column     TEXT NOT NULL,
    default_value   TEXT,                           -- literal SQL value (NULL kalau pakai source)
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE,
    UNIQUE (connection_id, table_name, dest_column)
);

CREATE INDEX IF NOT EXISTS idx_mappings_conn_table
    ON sync_column_mappings(connection_id, table_name);

-- Checkpoint untuk resume
CREATE TABLE IF NOT EXISTS sync_checkpoints (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id         INTEGER NOT NULL,
    table_name            TEXT NOT NULL,
    last_batch_completed  INTEGER NOT NULL DEFAULT 0,
    last_pk_value         TEXT NOT NULL DEFAULT '0', -- TEXT karena PK bisa string/int
    started_at            DATETIME NOT NULL,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status                TEXT NOT NULL,            -- running | interrupted | completed | failed
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE,
    UNIQUE (connection_id, table_name)              -- 1 checkpoint aktif per tabel
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_status
    ON sync_checkpoints(status);

-- Audit trail / history
CREATE TABLE IF NOT EXISTS sync_history (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id     INTEGER NOT NULL,
    table_name        TEXT NOT NULL,
    started_at        DATETIME NOT NULL,
    finished_at       DATETIME,
    duration_seconds  INTEGER,
    total_rows        INTEGER,
    success_rows      INTEGER,
    failed_rows       INTEGER,
    status            TEXT NOT NULL,                -- completed | failed | interrupted
    error_summary     TEXT,
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_history_conn_table
    ON sync_history(connection_id, table_name, started_at DESC);
```

**Catatan desain:**
- `connections` menyimpan **pair source+dest** dalam 1 row untuk simplifikasi v1 (1 connection alias = 1 sync target pair). Kalau kelak perlu reuse source ke banyak dest, refactor split ke 2 tabel.
- `last_pk_value` disimpan sebagai `TEXT` karena PK bisa INT/BIGINT/UUID/string — parsing tergantung tipe kolom saat runtime.
- `sync_checkpoints` UNIQUE per (connection, table) → checkpoint lama di-overwrite saat sync baru start; history-nya tetap di `sync_history`.
- `ON DELETE CASCADE` supaya hapus connection ikut bersih mappings/checkpoints/history.

---

## 6. Security Model

### 6.1 Credential Storage
- Password MySQL (source & dest) **TIDAK PERNAH** disimpan plaintext di SQLite.
- Format storage: `base64(AES-256-GCM(plaintext, key, nonce))` dengan nonce di-prepend.
- Encryption key derivation:
  1. **Master password mode (default):** prompt sekali per session, derive key via scrypt (N=32768, r=8, p=1, keyLen=32) dengan salt random per-install yang disimpan di `~/.config/dbsync/salt`.
  2. **Env var mode (CI/cron):** kalau `DBSYNC_MASTER_KEY` set, gunakan langsung (must be 32 bytes hex-encoded).

### 6.2 Master password lifecycle
- TUI: prompt saat startup, hold di memory selama session.
- CLI: cek env var `DBSYNC_MASTER_KEY` dulu; kalau tidak ada, prompt via stdin (cocok untuk one-shot run); kalau stdin non-interactive (cron), fail dengan instruction.
- Tidak ada disk caching key.

### 6.3 Log redaction
- Error log JSON lines: query parameters di-redact (`?` masked) — hanya struktur SQL yang dilog, bukan nilai concrete row.
- Connection string di log: password field selalu `***`.

---

## 7. Module Responsibilities

### 7.1 `internal/storage`
- Open SQLite, run migrations idempotently
- CRUD untuk 4 tabel
- Tidak tahu apa-apa soal MySQL atau bubbletea — pure data layer

### 7.2 `internal/crypto`
- 2 fungsi public: `Encrypt(plaintext, key) (ciphertext, error)`, `Decrypt(ciphertext, key) (plaintext, error)`
- 1 helper: `DeriveKey(masterPassword, salt) []byte`
- Tidak tahu soal SQLite atau MySQL

### 7.3 `internal/mysql`
- `Pool` wrapper: open `*sql.DB` dengan config (10 max open, 5 idle, 5min lifetime, 30s timeout)
- `DetectPK(db, schema, table) ([]string, error)` — query INFO_SCHEMA
- `DescribeColumns(db, schema, table) ([]Column, error)` — buat schema validation
- `SelectBatch(db, table, pkCols, lastPK, limit) ([]Row, error)`
- `Upsert(tx, table, mapping, rows) (n int, error)`

### 7.4 `internal/engine`
- `engine.Run(ctx, opts) (<-chan Event, error)` — orchestrator, emit event lewat channel
- `Event` types: `ProgressEvent`, `BatchErrorEvent`, `RowErrorEvent`, `DoneEvent`
- Tidak tahu apa frontend-nya (TUI/CLI sama-sama consume channel)

### 7.5 `internal/tui`
- bubbletea models, semua message lewat tea.Cmd
- Subscribe ke engine event channel via tea.Cmd polling
- Tidak boleh hold koneksi MySQL langsung — selalu via engine

### 7.6 `internal/cli`
- Parse flags (`--connection`, `--table`, `--all-tables`, `--dry-run`)
- Print progress ke stdout (tabular / oneline), tidak pakai bubbletea
- Exit code: 0 success, 1 partial fail, 2 fatal

---

## 8. Error Handling & Logging

### 8.1 Klasifikasi error
| Type | Action | UI |
|---|---|---|
| Connection fail (preflight) | Abort sebelum sync | Red banner, suggest "test connection" |
| Schema mismatch — column missing di source | Warning, lanjut (skip column) | Yellow banner, log line |
| Schema mismatch — type incompatible | Error, abort | Red banner |
| Row error (constraint, type) | Stop batch, rollback batch, log, abort sync | Error panel + log file |
| Batch error (network blip) | Same as row error — no retry di v1 | Error panel + log file |

### 8.2 Log file
- Path: `~/.local/share/dbsync/logs/sync-{timestamp}-{connection}-{table}.jsonl`
- Format JSON lines, satu line per event
- Field minimal: `{timestamp, level, batch, row_pk, error, sql_template}` (no raw values)

---

## 9. Dependencies

| Package | Versi target | Alasan |
|---|---|---|
| `github.com/charmbracelet/bubbletea` | latest stable | TUI framework |
| `github.com/charmbracelet/lipgloss` | latest stable | Styling |
| `github.com/charmbracelet/bubbles` | latest stable | Reusable components (textinput, table, spinner) |
| `github.com/go-sql-driver/mysql` | latest stable | MySQL driver |
| `modernc.org/sqlite` | latest stable | SQLite driver (pure-Go, no CGo) — pilihan final untuk cross-compile mudah |
| `github.com/spf13/cobra` | latest stable | CLI parsing |
| `golang.org/x/crypto` | latest stable | scrypt KDF |
| `golang.org/x/term` | latest stable | secure password prompt |

**SQLite driver:** Final = `modernc.org/sqlite` (pure-Go). Trade-off accepted: ~5-15% lebih lambat dari CGo, tapi cross-compile tanpa toolchain dan deployment binary tunggal lebih mudah.

---

## 10. Open Questions / Future Work

- **Multi-table parallel sync** — saat ini sequential; bisa di-parallel kalau dibutuhkan (v2).
- **Retry policy** — v1 fail-fast; v2 mungkin exponential backoff per batch.
- **Schema diff tool** — bantu user lihat mismatch sebelum sync (v2).
- **Bi-directional / conflict resolution** — explicitly out of scope.
- **Webhook / notification** — out of scope.
- **Pure-Go SQLite** — evaluasi saat scaffolding.

---

## 11. Glossary

- **Upsert** — `INSERT ... ON DUPLICATE KEY UPDATE` (MySQL); INSERT kalau PK belum ada, UPDATE kalau sudah.
- **Checkpoint** — row di `sync_checkpoints` yang menyimpan posisi terakhir batch berhasil; basis untuk resume.
- **Mapping** — pemetaan `source_column → dest_column` + optional `default_value`.
- **Preflight** — semua check sebelum batch loop dimulai (test conn, detect PK, validate schema).
- **God node** — (graphify) node dengan banyak relasi; bukan istilah app.

---

*End of document.*
