# Issue 003 — Column mapping CRUD

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 002
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 8, 9, 10, 11

---

## What to build

Mapping kolom source → dest per (connection, table). Auto-generate 1:1
default, override rename, set default value untuk kolom dest yang tidak
ada di source, dan warning untuk kolom dest `NOT NULL` tanpa mapping.

CLI baru: `dbsync mapping auto`, `mapping list`, `mapping set`, `mapping rm`.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`modernc.org/sqlite`** — topik: "transactions, parameterized queries
   with named parameters, batch insert performance".
2. **`database/sql`** — topik: "TxOptions isolation level, deferred rollback pattern".
3. **`github.com/spf13/cobra`** — topik: "subcommand with required and
   optional flags, custom validation".

---

## Step-by-step implementation

### Step 1 — `internal/storage/mappings.go`

```go
type Mapping struct {
    ID            int64
    ConnectionID  int64
    TableName     string
    SourceColumn  sql.NullString // nullable per PRD case 2
    DestColumn    string
    DefaultValue  sql.NullString // nullable per PRD case 1
    CreatedAt     time.Time
}
```

Method `MappingRepo`:
- `Insert(ctx, m Mapping) (int64, error)`
- `BulkInsert(ctx, ms []Mapping) error` — satu transaksi.
- `ListByTable(ctx, connID int64, table string) ([]Mapping, error)`
- `Upsert(ctx, m Mapping) error` — pakai `INSERT ... ON CONFLICT (connection_id, table_name, dest_column) DO UPDATE`.
- `Delete(ctx, id int64) error`
- `DeleteByTable(ctx, connID int64, table string) error`
- `Exists(ctx, connID int64, table string) (bool, error)` — apakah ada mapping untuk tabel.

**Constraint validation di layer storage:** sebelum insert, return error
kalau `SourceColumn` dan `DefaultValue` keduanya NULL (mapping kosong
tidak masuk akal).

Tests pakai `:memory:`:
- BulkInsert N row → ListByTable return N row urutan deterministic.
- Upsert sama (conn, table, dest) → update, bukan duplicate.
- UNIQUE constraint `(connection_id, table_name, dest_column)` enforced.
- FK CASCADE: delete connection → mappings ikut terhapus (uji via
  `connections_test.go` extension atau test khusus di `mappings_test.go`).
- Validation: insert dengan source NULL + default NULL → error.

### Step 2 — Helper auto-mapping

File: `internal/storage/mappings.go` (atau `automap.go` jika lebih besar).

```go
type AutoMapResult struct {
    Mappings        []Mapping     // mapping yang di-generate
    Warnings        []string      // dest NOT NULL kolom yang tidak ada di source dan tidak punya default
    UnmappedSource  []string      // kolom source yang tidak ada di dest (informational)
}

func AutoMap(connID int64, table string, sourceCols, destCols []mysql.Column) AutoMapResult
```

Logika:
- Untuk tiap kolom di dest:
  - Cari nama exact match di source → bikin mapping `{source: X, dest: X}`.
  - Kalau tidak ada match dan dest `NOT NULL` tanpa default DB → append warning.
- Untuk tiap kolom source yang tidak terpakai → append ke `UnmappedSource`.

Function ini **pure** (no DB), gampang di-unit-test dengan table-driven test.

Tests:
- All-match: source=[id,name,email], dest=[id,name,email] → 3 mapping, 0 warning.
- Extra dest column NOT NULL: source=[id,name], dest=[id,name,tenant_id NOT NULL] → 2 mapping + 1 warning.
- Extra dest column nullable: 2 mapping + 0 warning (dest column NULL-able).
- Extra source column: 1 unmapped source informational.

### Step 3 — CLI commands

File: `internal/cli/mapping.go`

`dbsync mapping auto --connection=NAME --table=TBL`:
- Resolve connection, connect source+dest, panggil `DescribeColumns` masing-masing.
- `AutoMap(...)`.
- Print preview (mapping count, warnings, unmapped source).
- Confirm prompt (kecuali `--yes`) → `BulkInsert`.
- Idempotent: kalau mapping sudah ada untuk (conn, table), tanya overwrite
  (`mapping rm` lalu insert), `--force` untuk skip prompt.

`dbsync mapping list --connection=NAME --table=TBL`:
- Print `SOURCE | DEST | DEFAULT`.
- `(NULL)` di kolom kalau nullable.

`dbsync mapping set --connection=NAME --table=TBL --dest=COL [--source=COL] [--default=VAL]`:
- Validasi: minimal salah satu `--source` atau `--default` harus ada.
- Upsert.

`dbsync mapping rm --connection=NAME --table=TBL [--dest=COL]`:
- Tanpa `--dest`: hapus semua mapping untuk tabel (confirm prompt).
- Dengan `--dest`: hapus 1 row.

### Step 4 — Warning surfacing

Kalau `mapping auto` menghasilkan warning, tampilkan di CLI:
```
⚠ 2 dest columns are NOT NULL and have no mapping (sync akan fail di runtime):
  - users.synced_at (TIMESTAMP NOT NULL)
  - users.tenant_id (INT NOT NULL)

Run: dbsync mapping set --connection=X --table=users --dest=synced_at --default='NOW()'
```

Pesan harus actionable — beri contoh command persis.

---

## Acceptance criteria

- [ ] `internal/storage.MappingRepo` lengkap dengan tests `:memory:` (CRUD, UNIQUE, FK CASCADE, validation source/default tidak boleh dua-duanya NULL).
- [ ] `storage.AutoMap` pure function dengan table-driven test (minimal 4 kasus).
- [ ] `dbsync mapping auto` generate mapping, print preview + warnings, confirm sebelum insert.
- [ ] `dbsync mapping list/set/rm` bekerja, idempotent.
- [ ] Re-run `mapping auto` di tabel yang sudah ada mapping → confirm prompt (atau `--force`).
- [ ] Warning message actionable (include command contoh).
- [ ] FK CASCADE diverifikasi: `dbsync conn rm <name>` menghapus semua mapping miliknya.
- [ ] `go test ./...` pass.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

## Blocked by

- Issue 002 (butuh `mysql.DescribeColumns` untuk auto-map, dan connectivity untuk verify schema).
