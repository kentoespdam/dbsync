# Issue 001 — Encrypted credential storage (crypto + config + storage minimal + CLI conn add/list)

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** None — can start immediately
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 1, 2, 4, 6, 24, 25

---

## What to build

Tracer-bullet pertama: bisa simpan koneksi MySQL ke SQLite dengan password
**terenkripsi** lalu list-back lagi. Sentuh seluruh stack vertikal:
crypto → config (master key) → storage (DB open + migration + connections repo)
→ CLI minimal (`dbsync conn add`, `dbsync conn list`).

Tidak ada MySQL connectivity di slice ini — itu Slice 002.

---

## REQUIRED: Use `context7` BEFORE writing code

Sebelum menulis kode di slice ini, **WAJIB** query `context7` MCP untuk
mengambil dokumentasi & best practice terbaru dari library yang dipakai.
Patuhi rules di `~/.claude/rules/context7.md`.

Query yang harus dijalankan (minimal):

1. `resolve-library-id` + `query-docs` untuk **`modernc.org/sqlite`** —
   topik: "open database, run migrations, foreign keys pragma, error handling".
2. `resolve-library-id` + `query-docs` untuk **`golang.org/x/crypto/scrypt`** —
   topik: "key derivation parameters N r p, memory cost, deriving 32-byte key".
3. `resolve-library-id` + `query-docs` untuk **`crypto/cipher` (Go stdlib AES-GCM)** —
   topik: "AES-256-GCM encrypt decrypt, nonce size, authentication tag".
4. `resolve-library-id` + `query-docs` untuk **`github.com/spf13/cobra`** —
   topik: "subcommand structure, persistent flags, error handling, exit codes".
5. `resolve-library-id` + `query-docs` untuk **`golang.org/x/term`** —
   topik: "ReadPassword from stdin, IsTerminal check".

Jangan mulai coding sampai dokumentasi terbaru sudah dibaca. Setelah
context7 selesai, gunakan API versi terbaru yang dikembalikan — jangan
pakai pattern dari memori internal yang mungkin sudah stale.

---

## Step-by-step implementation

### Step 1 — `internal/crypto` (pure, no I/O)

File: `internal/crypto/crypto.go`

- `func DeriveKey(masterPassword string, salt []byte) ([]byte, error)`
  - scrypt N=32768, r=8, p=1, keyLen=32.
  - Validasi: salt minimal 16 byte.
- `func Encrypt(plaintext, key []byte) (string, error)`
  - AES-256-GCM, nonce random 12 byte (gunakan `crypto/rand`).
  - Output format: `base64.StdEncoding.EncodeToString(nonce || ciphertext_with_tag)`.
  - Validasi: key panjang 32 byte.
- `func Decrypt(b64 string, key []byte) ([]byte, error)`
  - Reverse dari `Encrypt`. Error spesifik kalau tag mismatch (tampered).

File: `internal/crypto/crypto_test.go` — **WAJIB**:
- Round-trip: `Encrypt(plaintext) → Decrypt → plaintext`.
- KDF determinism: same (password, salt) → same key (run 2x bandingkan).
- Tamper detection: flip 1 byte di ciphertext → `Decrypt` return error.
- Nonce uniqueness: encrypt input sama 1000x → 1000 ciphertext unik
  (cek dengan map[string]bool).
- Key length validation: pass key 16 byte → error.

### Step 2 — `internal/config` (master key lifecycle)

File: `internal/config/config.go`

- `func LoadMasterKey(ctx context.Context) ([]byte, error)`
  - **Order of operations:**
    1. Cek env var `DBSYNC_MASTER_KEY`. Kalau ada: validasi panjang
       64 char hex, decode → return raw 32 byte. Kalau invalid → error
       `"DBSYNC_MASTER_KEY must be 64 hex characters (32 bytes)"`.
    2. Kalau env tidak set: cek `term.IsTerminal(int(os.Stdin.Fd()))`.
       - Kalau interactive: prompt password 2x (confirm jika file salt
         belum ada → first setup), 1x kalau salt sudah ada. Pakai
         `term.ReadPassword`.
       - Kalau non-interactive: return error
         `"DBSYNC_MASTER_KEY not set and stdin is not a terminal. Set the env var or run dbsync interactively."`
    3. Load atau generate salt di `~/.config/dbsync/salt` (16 byte
       random, mode 0600). Direktori dibuat dengan `os.MkdirAll(dir, 0700)`.
    4. Call `crypto.DeriveKey(password, salt)` → return key.
- `func SaltPath() (string, error)` — resolve path (XDG fallback).

Test (medium priority, skip integration):
- Env var path: set env → decode benar.
- Env var invalid length → error.
- Salt file auto-create di tempdir.

### Step 3 — `internal/storage` (DB + connections repo)

File: `internal/storage/db.go`

- `func Open(dbPath string) (*DB, error)`
  - `sql.Open("sqlite", dbPath)`.
  - `db.Exec("PRAGMA foreign_keys = ON;")` — **wajib**, jangan lupa.
  - Jalankan migration `001_init.sql` (embed pakai `//go:embed migrations/*.sql`).
  - Migration idempotent (file pakai `CREATE TABLE IF NOT EXISTS`).
- `func (d *DB) Close() error`.
- Struct `DB` membungkus `*sql.DB` dan expose `Connections() *ConnectionRepo`.

File: `internal/storage/connections.go`

Tipe domain (struct di package `storage`):
```go
type Connection struct {
    ID             int64
    Name           string
    SourceHost     string
    SourcePort     int
    SourceUser     string
    SourcePassword string // already ciphertext (base64), not plaintext
    SourceDB       string
    DestHost       string
    DestPort       int
    DestUser       string
    DestPassword   string // already ciphertext (base64)
    DestDB         string
    CreatedAt      time.Time
    UpdatedAt      time.Time
}
```

Repo method:
- `Insert(ctx, c Connection) (int64, error)`
- `GetByName(ctx, name string) (Connection, error)` — `sql.ErrNoRows` mapped ke sentinel `ErrNotFound`.
- `GetByID(ctx, id int64) (Connection, error)`
- `List(ctx) ([]Connection, error)`
- `Update(ctx, c Connection) error`
- `Delete(ctx, id int64) error`

**Penting:** repo TIDAK pegang master key. Caller (CLI/TUI) yang
encrypt password sebelum `Insert`, decrypt setelah `Get*`.

File: `internal/storage/connections_test.go` — **WAJIB**:
- Pakai `:memory:` SQLite.
- CRUD round-trip per method.
- UNIQUE constraint pada `name` (insert dua kali nama sama → error).
- Migration idempotent: jalankan Open 2x di file yang sama → no error.
- FK CASCADE (akan dipakai di slice lain; tes minimal saja di sini).

### Step 4 — CLI minimal (`dbsync conn`)

File: `internal/cli/cli.go` — cobra root + wire `conn` subcommand.
File: `internal/cli/conn.go` — `conn add`, `conn list`, `conn rm`.

Behavior:
- `dbsync conn add` — interactive: tanya name, source host/port/user/db,
  source password (pakai `term.ReadPassword`), dest host/port/user/db,
  dest password. Encrypt 2 password dengan master key dari
  `config.LoadMasterKey`. Insert ke DB.
- `dbsync conn list` — print table: `NAME | SOURCE | DEST | CREATED`.
  Password jangan pernah ditampilkan; host:port saja.
- `dbsync conn rm <name>` — confirm prompt, lalu delete.

Wire `runCLI` di `cmd/dbsync/main.go` ke `cli.Execute(args)`.

DB path resolution: `~/.local/share/dbsync/dbsync.db` (XDG_DATA_HOME).
Auto-create direktori dengan mode 0700.

### Step 5 — Wire & smoke test

- `go build -o dbsync ./cmd/dbsync`
- `DBSYNC_MASTER_KEY=$(openssl rand -hex 32) ./dbsync conn add` (manual smoke).
- `./dbsync conn list` → row muncul, password tidak bocor.
- Inspect SQLite: `source_password` dan `dest_password` adalah base64
  ciphertext, bukan plaintext.

---

## Acceptance criteria

- [ ] `internal/crypto` package: `DeriveKey`, `Encrypt`, `Decrypt` lengkap dengan tests (round-trip, KDF determinism, tamper detection, nonce uniqueness, key length validation) — semua test pass.
- [ ] `internal/config.LoadMasterKey` handles 3 paths: env var (valid + invalid length), interactive prompt, non-interactive failure. Salt file auto-generate di `~/.config/dbsync/salt`.
- [ ] `internal/storage.Open` jalankan migration idempotent dan enable `PRAGMA foreign_keys = ON`.
- [ ] `internal/storage.ConnectionRepo` CRUD lengkap dengan tests pakai `:memory:` SQLite. UNIQUE name constraint diuji.
- [ ] CLI `dbsync conn add`, `dbsync conn list`, `dbsync conn rm` jalan. Password TIDAK pernah disimpan plaintext di DB (verify via `sqlite3 dbsync.db "SELECT source_password FROM connections"` → base64 string).
- [ ] `DBSYNC_MASTER_KEY` env var path bekerja non-interaktif (cocok untuk cron).
- [ ] Non-interactive tanpa env var → exit dengan pesan jelas, exit code ≠ 0.
- [ ] `go vet ./...` dan `go test ./...` pass.
- [ ] `context7` MCP sudah di-query untuk semua library di atas SEBELUM coding (catat di PR description).

## Blocked by

None — can start immediately.
