# Issue 005 — Checkpoint, resume, history, `--all-tables`, `--dry-run`

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 004
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 15, 16, 17, 18, 19, 20 (data layer), 23 (data layer)

---

## What to build

Engine matang: loop batch sampai habis, checkpoint per batch, resume
dari checkpoint, idempotent re-run, audit history per run, CLI flag
`--all-tables` dan `--dry-run`, exit code semantik 0/1/2 final.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`modernc.org/sqlite`** — topik: "upsert / INSERT OR REPLACE,
   transaction isolation, WAL mode performance".
2. **`database/sql`** — topik: "context cancellation propagation,
   Tx.Rollback semantic, ErrTxDone".
3. **`github.com/spf13/cobra`** — topik: "flag groups, mutually
   exclusive flags (`MarkFlagsMutuallyExclusive`)".

---

## Step-by-step implementation

### Step 1 — `internal/storage/checkpoints.go`

```go
type Checkpoint struct {
    ID                  int64
    ConnectionID        int64
    TableName           string
    LastBatchCompleted  int
    LastPKValue         string // serialized (single PK as plain string, composite as JSON array)
    StartedAt           time.Time
    UpdatedAt           time.Time
    Status              string // 'running' | 'interrupted' | 'completed' | 'failed'
}
```

Method `CheckpointRepo`:
- `Get(ctx, connID int64, table string) (Checkpoint, error)` — return `ErrNotFound` kalau belum ada.
- `Upsert(ctx, c Checkpoint) error` — `INSERT ... ON CONFLICT(connection_id, table_name) DO UPDATE`.
- `MarkInterrupted(ctx, connID, table)` — set status di running checkpoint.
- `MarkCompleted(ctx, connID, table)`.
- `MarkFailed(ctx, connID, table)`.
- `ListActive(ctx) ([]Checkpoint, error)` — status `running` atau `interrupted`.
- `Delete(ctx, connID int64, table string) error` — untuk reset.

Tests `:memory:`:
- Upsert dua kali (conn, table) sama → 1 row, status terupdate.
- UNIQUE constraint enforced.
- ListActive return hanya `running`/`interrupted`.

### Step 2 — `internal/storage/history.go`

```go
type HistoryRecord struct {
    ID              int64
    ConnectionID    int64
    TableName       string
    StartedAt       time.Time
    FinishedAt      sql.NullTime
    DurationSeconds sql.NullInt64
    TotalRows       sql.NullInt64
    SuccessRows     sql.NullInt64
    FailedRows      sql.NullInt64
    Status          string // 'running' | 'completed' | 'failed' | 'interrupted'
    ErrorSummary    sql.NullString
}
```

Method `HistoryRepo`:
- `Begin(ctx, connID int64, table string) (int64, error)` — insert row status `running`, return id.
- `Finish(ctx, id int64, h HistoryRecord) error` — update final fields.
- `List(ctx, connID int64, table string, limit int) ([]HistoryRecord, error)` — order by `started_at DESC`.
- `Latest(ctx, connID int64, table string) (HistoryRecord, error)`.

Tests `:memory:`:
- Begin lalu Finish → row final benar.
- List urutan descending.
- FK CASCADE: delete connection → history terhapus.

### Step 3 — Engine: full batch loop + checkpoint

Update `internal/engine/engine.go`:

Algoritma `Run`:
1. Load connection, mappings, key. Detect PK di source.
2. **Resume check:**
   - `checkpoints.Get(conn, table)` →
     - `running` → catat warning "previous run not finalized, treating as interrupted".
     - `interrupted` → resume dari `LastPKValue`. TUI prompt (Issue 008), CLI auto-resume.
     - `completed` → start fresh dengan `LastPKValue = sentinel`.
     - `failed` → start fresh.
     - `ErrNotFound` → start fresh.
3. `history.Begin` → dapat history_id.
4. Update checkpoint status `running`, `StartedAt = now`.
5. Loop batch:
   ```go
   for {
       batchN++
       rows, nextPK, err := mysql.SelectBatch(ctx, src, schema, table, pkCols, lastPK, batchSize)
       if err != nil { /* fail handling */ }
       if len(rows) == 0 { break } // done

       if opts.DryRun {
           progress.Rows += len(rows)
           emit ProgressEvent
           lastPK = nextPK
           continue
       }

       tx, _ := dest.BeginTx(ctx, nil)
       tx.Exec("SET FOREIGN_KEY_CHECKS=0")
       _, err = mysql.Upsert(ctx, tx, destSchema, destTable, mappings, rows)
       if err != nil {
           tx.Rollback()
           logger.BatchError(batchN, err, sqlTemplate)
           emit BatchErrorEvent
           // fail-fast per PRD
           goto done
       }
       if err := tx.Commit(); err != nil { /* same */ }

       // Update checkpoint atomic
       checkpoints.Upsert(ctx, Checkpoint{
           ConnectionID: connID, TableName: table,
           LastBatchCompleted: batchN,
           LastPKValue: serializePK(nextPK),
           Status: "running",
       })
       lastPK = nextPK
       totalRows += len(rows)
       emit ProgressEvent
   }
   done:
   ```
6. Finalize:
   - Sukses → `checkpoints.MarkCompleted`, `history.Finish` status `completed`.
   - Fatal → `checkpoints.MarkFailed`, `history.Finish` status `failed`.
   - Context canceled → `checkpoints.MarkInterrupted`, `history.Finish` status `interrupted`.
   - Emit `DoneEvent`.

**Idempotency:** upsert + checkpoint by-PK menjamin re-run sync yang
sudah selesai = no-op (semua row sudah di dest dengan PK yang sama).

**PK serialization** untuk composite:
```go
func serializePK(values []any) string {
    if len(values) == 1 { return fmt.Sprint(values[0]) }
    b, _ := json.Marshal(values)
    return string(b)
}
```
Dan deserialize sebaliknya saat resume (butuh PK type info dari
`DescribeColumns`).

### Step 4 — Dry-run mode

Saat `opts.DryRun == true`:
- Tidak panggil `Upsert`, tidak buka transaksi di dest.
- Tetap connect dest untuk verify reachable.
- Tetap loop `SelectBatch` untuk hitung total rows.
- TIDAK tulis checkpoint atau history (atau tulis dengan flag `dry_run=true`
  di `ErrorSummary` — pilih: skip saja untuk v1 supaya lebih simple).
- Emit `ProgressEvent` normal.
- `DoneEvent.TotalRows` = estimasi row yang akan di-sync.

### Step 5 — CLI flag `--all-tables` dan `--dry-run`

File: `internal/cli/run.go`

```
dbsync run --connection=NAME --table=TBL [--dry-run] [--batch-size=N]
dbsync run --connection=NAME --all-tables [--dry-run]
```

Mutually exclusive: `--table` dan `--all-tables` (gunakan
`cmd.MarkFlagsMutuallyExclusive("table", "all-tables")`).

`--all-tables`:
- Query `mappings` table → `SELECT DISTINCT table_name WHERE connection_id = ?`.
- Loop tabel: panggil `engine.Run` satu per satu (sequential, sesuai PRD).
- Aggregate hasil: `success_count`, `partial_fail_count`, `fatal_count`.
- Exit code:
  - Semua sukses → 0.
  - Sebagian gagal (1+ tabel fail tapi 1+ tabel sukses) → 1.
  - Connection error, master key error, schema mismatch → 2.

### Step 6 — CLI `dbsync history` dan `dbsync checkpoints`

Bonus (sekalian karena data layer sudah ada):

`dbsync history --connection=NAME [--table=TBL] [--limit=10]`:
- Print: `STARTED | DURATION | TABLE | ROWS | STATUS`.

`dbsync checkpoints list` — print semua aktif (running/interrupted).
`dbsync checkpoints reset --connection=NAME --table=TBL` — `checkpoints.Delete`.

---

## Acceptance criteria

- [ ] `CheckpointRepo` & `HistoryRepo` lengkap dengan tests `:memory:` (CRUD, UNIQUE checkpoint, FK CASCADE history).
- [ ] Engine loop batch sampai habis, checkpoint terupdate per batch sukses.
- [ ] Resume: kill mid-sync (SIGINT), checkpoint status `interrupted`, run ulang → mulai dari `LastPKValue`, total row akhir = total row source.
- [ ] Idempotent re-run: jalankan sync yang sudah `completed` 2x → tidak ada duplicate, tidak ada error, total row dest tidak berubah.
- [ ] `--dry-run` print estimasi row tanpa upsert (verify: row dest tidak bertambah setelah dry-run).
- [ ] `--all-tables` sync semua tabel ber-mapping sequential. Exit code 0/1/2 sesuai PRD.
- [ ] Composite PK resume bekerja (uji dengan tabel PK (a, b)).
- [ ] SIGINT di tengah sync → checkpoint `interrupted`, history `interrupted`, exit 130.
- [ ] `dbsync history` dan `dbsync checkpoints` jalan.
- [ ] Integration test pakai testcontainers: sync 5000 row, kill di batch 3, resume, verify row count.
- [ ] `go test ./... -tags=integration` pass.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

## Blocked by

- Issue 004 (butuh engine MVP, mysql.Upsert/SelectBatch, logger).
