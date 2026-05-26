# Issue 002 — MySQL connectivity & schema inspection

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 001
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 3, 7

---

## What to build

Bisa konek ke source/dest MySQL pakai koneksi yang disimpan di Issue 001,
list tabel di source DB, dan deteksi primary key (single + composite).
Tracer-bullet kedua: storage → crypto (decrypt) → mysql pool → CLI.

CLI baru: `dbsync conn test <name>` dan `dbsync tables list --connection=<name>`.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/go-sql-driver/mysql`** — topik: "DSN format with timeout
   and tls, connection pool sizing, error sentinel values, parseTime,
   readTimeout writeTimeout".
2. **`database/sql` (Go stdlib)** — topik: "DB.SetMaxOpenConns,
   SetMaxIdleConns, SetConnMaxLifetime, PingContext, query INFORMATION_SCHEMA".
3. **`github.com/spf13/cobra`** — topik: "required flags, flag inheritance,
   command groups".

Jangan coding sebelum dokumentasi dibaca.

---

## Step-by-step implementation

### Step 1 — `internal/mysql/pool.go`

Tipe:
```go
type Config struct {
    Host     string
    Port     int
    User     string
    Password string // plaintext
    DBName   string
}

type Pool struct {
    db *sql.DB
}
```

Functions:
- `func Open(cfg Config) (*Pool, error)`
  - Build DSN: `user:pass@tcp(host:port)/dbname?parseTime=true&timeout=30s&readTimeout=30s&writeTimeout=30s&charset=utf8mb4`.
  - `sql.Open("mysql", dsn)`.
  - Pool tuning: `SetMaxOpenConns(10)`, `SetMaxIdleConns(5)`, `SetConnMaxLifetime(5 * time.Minute)`.
  - `PingContext(ctx)` dengan timeout 5s untuk verify reachable.
- `func (p *Pool) Close() error`
- `func (p *Pool) DB() *sql.DB` — escape hatch buat test, hindari di production code.

**Catatan keamanan:** kalau `Open` gagal dan error message mengandung
DSN, **harus** redact password jadi `***` sebelum bubble up. Gunakan
helper `redactDSN(dsn)`.

### Step 2 — `internal/mysql/schema.go`

```go
type Column struct {
    Name     string
    DataType string // e.g. "varchar", "int", "bigint"
    IsNullable bool
    ColumnKey  string // "PRI", "UNI", "MUL", ""
}
```

Functions:
- `func DetectPK(ctx, db *sql.DB, schema, table string) ([]string, error)`
  - Query:
    ```sql
    SELECT COLUMN_NAME
    FROM INFORMATION_SCHEMA.KEY_COLUMN_USAGE
    WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
    ORDER BY ORDINAL_POSITION
    ```
  - Return `[]string` urutan kolom (composite PK bisa multi-kolom).
  - Empty slice + nil error kalau tabel tanpa PK (caller decide).
- `func DescribeColumns(ctx, db *sql.DB, schema, table string) ([]Column, error)`
  - Query `INFORMATION_SCHEMA.COLUMNS` ordered by `ORDINAL_POSITION`.
- `func ListTables(ctx, db *sql.DB, schema string) ([]string, error)`
  - Query `INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA=? AND TABLE_TYPE='BASE TABLE'`.

### Step 3 — Tests pakai testcontainers

File: `internal/mysql/schema_test.go`

Pattern:
```go
//go:build integration

func TestDetectPK(t *testing.T) {
    if os.Getenv("MYSQL_TEST_DSN") == "" && !dockerAvailable() {
        t.Skip("set MYSQL_TEST_DSN or run with Docker")
    }
    // testcontainers mysql:8 boot, exec DDL, run assertions
}
```

Kasus test:
- Single-column PK → return `["id"]`.
- Composite PK (e.g. `PRIMARY KEY (tenant_id, user_id)`) → return
  `["tenant_id", "user_id"]` (order preserved).
- Tabel tanpa PK → return `[]string{}`, no error.
- `DescribeColumns`: jumlah kolom benar, `IsNullable` benar, `ColumnKey`
  populated.
- `ListTables`: tabel test ada di hasil, system tables tidak ada.

Tag `//go:build integration` supaya `go test ./...` default skip.
Tambahkan target di `Makefile` (atau dokumentasi README) untuk
`go test -tags=integration ./internal/mysql/...`.

### Step 4 — CLI baru

File: `internal/cli/conn_test_cmd.go`

`dbsync conn test <name>`:
- `LoadMasterKey` → load Connection → decrypt source_password & dest_password.
- `mysql.Open` untuk source, ping → print `source OK (mysql 8.x)`.
- `mysql.Open` untuk dest, ping → print `dest OK`.
- Exit 0 success, 2 kalau salah satu fail. Error message redacted.

File: `internal/cli/tables.go`

`dbsync tables list --connection=<name>`:
- Decrypt source creds → connect → `mysql.ListTables`.
- Untuk tiap tabel: panggil `DetectPK` → tampilkan `TABLE | PK_COLUMNS`.
- Format output: tabular, sortir alfabetis.

### Step 5 — Integrasi `conn add` dengan test connection

Update `dbsync conn add` (dari Issue 001) supaya **default** test
connection sebelum simpan. Flag `--no-test` untuk skip. Kalau test gagal,
print error dan tanya konfirmasi simpan tetap (untuk kasus host belum
ready).

---

## Acceptance criteria

- [ ] `internal/mysql.Pool` open + ping + close bekerja dengan pool tuning sesuai PRD (10/5/5min/30s).
- [ ] `DetectPK` support single & composite PK, urutan kolom konsisten dengan `ORDINAL_POSITION`.
- [ ] `DescribeColumns` return semua kolom dengan `IsNullable` dan `ColumnKey` benar.
- [ ] `ListTables` exclude views dan system tables.
- [ ] DSN dengan password TIDAK pernah leak ke error log (verify via test).
- [ ] Integration test pakai testcontainers `mysql:8` lengkap (skip kalau Docker tidak ada).
- [ ] `dbsync conn test <name>` jalan: success print OK, failure exit 2 dengan pesan redacted.
- [ ] `dbsync tables list --connection=<name>` print tabel + PK detected.
- [ ] `dbsync conn add` test koneksi otomatis (dengan `--no-test` opt-out).
- [ ] `context7` MCP di-query untuk semua library di atas SEBELUM coding (catat di PR description).

## Blocked by

- Issue 001 (butuh `internal/storage`, `internal/crypto`, `internal/config`, dan CLI plumbing dari sana).
