# Issue 004 — Single-batch sync tracer (engine MVP + JSONL logger)

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 003
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 14, 15 (partial), 21, 22

---

## What to build

Tracer-bullet sync end-to-end TANPA checkpoint / resume / dry-run /
all-tables. Ambil 1 batch (1000 row) dari source, upsert ke dest dengan
mapping yang sudah disimpan, log error ke JSONL. CLI minimal:
`dbsync run --connection=NAME --table=TBL` exit 0/2.

Checkpoint & history full akan ditambahkan di Issue 005.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/go-sql-driver/mysql`** — topik: "ON DUPLICATE KEY UPDATE
   syntax, SET FOREIGN_KEY_CHECKS, transaction with multi-row insert,
   prepared statements vs string interpolation, time.Time scan".
2. **`database/sql`** — topik: "Rows.Scan into []interface{}, dynamic
   column handling, sql.RawBytes, NullTime".
3. **`encoding/json` (stdlib)** — topik: "json.Encoder for line-delimited
   output, streaming write".
4. **Go stdlib `log/slog`** — topik: "structured logging with JSON
   handler, custom levels" (opsional, kalau pakai slog untuk JSONL writer).

---

## Step-by-step implementation

### Step 1 — `internal/mysql/ops.go`

Tipe:
```go
type Row map[string]any // column name → value (raw from driver)
```

Functions:
- `func SelectBatch(ctx, db *sql.DB, schema, table string, pkCols []string, lastPK []any, limit int) (rows []Row, nextPK []any, err error)`
  - Query: `SELECT * FROM <schema>.<table> WHERE (pk1, pk2, ...) > (?, ?, ...) ORDER BY pk1, pk2 LIMIT ?`.
  - Untuk single PK, simplify ke `WHERE pk > ?`.
  - Composite PK: pakai row-value comparison `(a, b) > (?, ?)` (MySQL 8 mendukung).
  - Scan ke `[]Row` dengan column discovery via `rows.Columns()`.
  - `nextPK` = PK values dari row terakhir di batch (untuk next iteration).
  - Empty batch → return `nil, nil, nil`.
  - Identifier quoting: pakai backtick \` \` untuk schema/table/column.
- `func Upsert(ctx, tx *sql.Tx, schema, table string, mappings []ResolvedMapping, rows []Row) (int, error)`
  - Build statement: `INSERT INTO \`schema\`.\`table\` (\`destCol1\`, \`destCol2\`, ...) VALUES (?, ?, ...), (?, ?, ...), ... ON DUPLICATE KEY UPDATE \`destCol1\` = VALUES(\`destCol1\`), ...`.
  - Skip primary key di clause `ON DUPLICATE KEY UPDATE` (jangan update PK; opsional optimasi).
  - Args slice flatten dari semua row.
  - Return `len(rows)` kalau sukses.

### Step 2 — Mapping resolver

File: `internal/engine/mapping.go`

```go
type ResolvedMapping struct {
    DestColumn   string
    ValueFn      func(row mysql.Row) (any, error) // produces value for this dest column
}

func Resolve(mappings []storage.Mapping) []ResolvedMapping
```

Logika per PRD 3 kasus:
1. `Source NOT NULL, Default NULL` → `ValueFn` ambil `row[source]`.
2. `Source NULL, Default NOT NULL` → `ValueFn` return literal default
   (parse: kalau `NOW()` → `time.Now()`, kalau angka → int64, else string).
3. `Both NOT NULL` (fallback): `row[source]`, kalau nil pakai default literal.

Tests table-driven (pure function, no DB):
- Case 1, 2, 3 di atas masing-masing.
- Default `NOW()` → return `time.Time` mendekati `time.Now()`.
- Default angka literal `"42"` → return `int64(42)`.
- Default string literal `"'hello'"` → strip quote → `"hello"`.

### Step 3 — `internal/logger/jsonl.go`

```go
type Logger struct {
    file *os.File
    enc  *json.Encoder
    mu   sync.Mutex
}

type Entry struct {
    Timestamp    time.Time `json:"timestamp"`
    Level        string    `json:"level"`        // "row_error" | "batch_error"
    Batch        int       `json:"batch"`
    RowPK        any       `json:"row_pk,omitempty"`
    Error        string    `json:"error"`
    SQLTemplate  string    `json:"sql_template,omitempty"`
}

func New(connectionName, tableName string) (*Logger, error)
func (l *Logger) RowError(batch int, pk any, err error, sqlTemplate string)
func (l *Logger) BatchError(batch int, err error, sqlTemplate string)
func (l *Logger) Close() error
```

Path: `~/.local/share/dbsync/logs/sync-<ts>-<conn>-<table>.jsonl`
(ts format `20060102-150405`). Auto-create direktori 0700.

**Redaction sangat penting:**
- `SQLTemplate` boleh disimpan (`INSERT INTO users (a, b) VALUES (?, ?)`).
- Argumen actual TIDAK boleh ditulis ke log.
- Error message dari MySQL driver kadang berisi nilai row — sanitize:
  jika error mengandung pattern `near '...'`, strip nilai di dalam
  quotes. Sediakan helper `sanitizeError(err) string`.

Tests:
- Write 3 row errors → file punya 3 baris JSON valid.
- Concurrent write (2 goroutine, 100 entry masing-masing) → 200 baris
  tanpa korupsi (mutex bekerja).
- `sanitizeError`: input `"Duplicate entry 'john@example.com' for key 'email'"` → output tanpa email.

### Step 4 — `internal/engine/engine.go`

```go
type Options struct {
    ConnectionID int64
    TableName    string
    BatchSize    int // default 1000
}

type Event interface{ isEvent() }
type ProgressEvent struct{ Batch, RowsDone int }
type BatchErrorEvent struct{ Batch int; Err error }
type RowErrorEvent  struct{ Batch int; PK any; Err error }
type DoneEvent      struct{ TotalRows int; Err error /* nil if success */ }

type Engine struct {
    store  *storage.DB
    crypto cryptoDecryptor // interface untuk mock
    logger *logger.Logger
}

func New(store *storage.DB, key []byte, log *logger.Logger) *Engine
func (e *Engine) Run(ctx context.Context, opts Options) (<-chan Event, error)
```

Behavior (Issue 004 scope only):
1. Load connection, decrypt password.
2. Load mappings untuk (conn, table). Kalau kosong → error.
3. Connect source+dest pool.
4. `DetectPK` di source untuk dapat PK columns.
5. Resolve mappings → `[]ResolvedMapping`.
6. Loop 1 iterasi saja (single batch):
   - Begin tx di dest, `SET FOREIGN_KEY_CHECKS=0`.
   - `SelectBatch` 1000 row pertama (lastPK = sentinel: `0` untuk numeric, `""` untuk string).
   - Build args via `ResolvedMapping`, `Upsert`.
   - Commit tx.
   - Emit `ProgressEvent` lalu `DoneEvent`.
7. Error policy: row error → log JSONL + emit `BatchErrorEvent`,
   rollback tx, emit `DoneEvent{Err: ...}`, return.

Channel buffered size 16. Consumer wajib drain sampai `DoneEvent`.

### Step 5 — Wire CLI `dbsync run`

File: `internal/cli/run.go`

`dbsync run --connection=NAME --table=TBL`:
- Resolve connection, key, logger.
- `engine.Run(ctx, opts)`.
- Loop consume event:
  - `ProgressEvent` → print `batch=N rows=M` (1 line replace pakai `\r`
    kalau stdout TTY, append kalau bukan).
  - `BatchErrorEvent` / `RowErrorEvent` → print ke stderr.
  - `DoneEvent` → print summary, set exit code (0 sukses, 2 fatal).
- Honor `SIGINT` (Ctrl+C): cancel context, tunggu `DoneEvent`, exit 130.

### Step 6 — Smoke test manual

```bash
# Setup: 2 MySQL container, schema sama, source punya data
DBSYNC_MASTER_KEY=$(openssl rand -hex 32)
./dbsync conn add  # interaktif
./dbsync mapping auto --connection=demo --table=users --yes
./dbsync run --connection=demo --table=users
# Verify: row di dest persis dengan source (sampai 1000 row pertama).
```

---

## Acceptance criteria

- [ ] `mysql.SelectBatch` benar untuk single PK & composite PK (uji manual + integration test).
- [ ] `mysql.Upsert` build statement `ON DUPLICATE KEY UPDATE` benar, idempotent (jalankan 2x → row count dest sama, no duplicate).
- [ ] `engine.Resolve` lengkap dengan 3 kasus mapping + table-driven test.
- [ ] `logger.JSONL` write line-delimited JSON; redaction berjalan (test sanitizeError).
- [ ] `engine.Run` emit event channel; consumer di CLI bisa subscribe dan terminate dengan DoneEvent.
- [ ] `dbsync run` exit 0 saat sukses, 2 saat fatal (connection / schema mismatch).
- [ ] SIGINT graceful: rollback batch in-flight, exit 130.
- [ ] Log file `~/.local/share/dbsync/logs/sync-*.jsonl` muncul setiap run, TIDAK berisi raw row values.
- [ ] `go test ./...` pass; integration test `mysql:8` testcontainers pass dengan `-tags=integration`.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

## Blocked by

- Issue 003 (butuh mapping data + storage repo).
