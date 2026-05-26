# dbsync — Project Context

> Quick-orientation file untuk agent baru / kolaborator. Bukan pengganti
> dokumentasi lengkap — lihat `docs/PRD-v1.md` dan `docs/ARCHITECTURE.md`
> untuk detail. File ini cuma rangkuman "apa, kenapa, di mana".

---

## 1. Apa ini?

`dbsync` adalah single-binary Go tool untuk **sinkronisasi tabel MySQL
satu arah** (source → destination). Tujuannya: bikin operasi yang
selama ini ad-hoc (`mysqldump | mysql`, shell script, klik GUI) jadi
**auditable, resumable, dan cron-friendly**, tanpa harus pasang
infrastruktur baru.

Dua mode operasi, satu binary, satu storage SQLite:

| Mode | Cara jalankan | Untuk apa |
|------|---------------|-----------|
| **TUI** | `./dbsync` | Setup koneksi, mapping kolom, sync interaktif dengan progress bar |
| **CLI** | `./dbsync run --connection=X --table=Y` | Cron / automation. Exit code semantik (0/1/2) |

Status saat ini: **v1, scaffolding phase.** Belum functional end-to-end.

---

## 2. Kenapa dibikin?

Lihat `docs/PRD-v1.md` §Problem Statement. Singkatnya, 5 pain point:

1. Tidak ada visibility riwayat sync.
2. Tidak resumable kalau gagal di tengah.
3. Tidak ada column mapping (rename, default value, kolom ekstra di dest).
4. Credential tersebar plaintext di shell script.
5. Tidak cocok untuk cron (butuh interaksi manual).

---

## 3. Prinsip desain (non-negotiable)

Dipanggil ulang di mana-mana — patuhi tanpa nego:

- **Simple > clever.** Tidak ada DI framework. Tidak ada abstraksi
  layered yang tidak perlu. Code harus terbaca junior dev + local AI.
- **SQLite = Single Source of Truth.** TUI dan CLI **tidak boleh** pegang
  state config in-memory yang divergen dari DB.
- **Frontend-agnostic core.** Engine emit `<-chan Event`; TUI dan CLI
  consume channel yang sama.
- **No CGo.** `modernc.org/sqlite` (pure Go) — supaya cross-compile gampang
  dari Mac → Linux tanpa Docker.
- **Test priority:** `crypto`, `storage`, `mysql`, `engine` **wajib** test.
  `cli` dan `tui` cukup manual QA.
- **Integration test:** tag dengan `//go:build integration` supaya
  `go test ./...` default tidak butuh Docker.

---

## 4. Layout

```
cmd/dbsync/         entry point (TUI vs CLI dispatch)
internal/
├── crypto/         AES-256-GCM + scrypt KDF (pure, no I/O)
├── config/         Master password lifecycle (env → stdin → salt file)
├── storage/        SQLite repo (connections, mappings, checkpoints, history)
├── mysql/          Pool + INFO_SCHEMA + batch select + upsert  ⟵ belum ada
├── engine/         Sync orchestrator, emit channel event       ⟵ belum ada
├── logger/         JSON-lines log writer dengan redaction      ⟵ belum ada
├── cli/            cobra command handlers
└── tui/            bubbletea models, views, update             ⟵ belum ada

docs/
├── PRD-v1.md              Product requirements
├── ARCHITECTURE.md        Design document (data flow, schema, security)
├── EXECUTION-ORDER.md     Roadmap tracer-bullet (8 issues)
└── issues/                Spec per-issue (001–008)
```

Yang sudah ada (Phase 1 scaffolding):
- `internal/crypto/` — encrypt/decrypt + KDF, sudah ada test
- `internal/config/` — master key loader, sudah ada test
- `internal/storage/` — `db.go`, `connections.go` + migration `001_init.sql`
- `internal/cli/` — skeleton `cli.go` + `conn.go`

---

## 5. Stack

- **Go 1.25** (modul `github.com/user/dbsync`)
- **TUI:** `charmbracelet/bubbletea` + `bubbles` + `lipgloss`
- **CLI:** `spf13/cobra`
- **MySQL driver:** `go-sql-driver/mysql`
- **SQLite:** `modernc.org/sqlite` (no CGo)
- **Crypto:** `golang.org/x/crypto/scrypt`, stdlib `crypto/cipher`
- **Term:** `golang.org/x/term`

---

## 6. Security model (ringkas)

- AES-256-GCM, nonce per ciphertext (random 12 byte, prepended).
- scrypt KDF: `N=32768, r=8, p=1, keyLen=32`.
- Salt random per-install di `~/.config/dbsync/salt`.
- Master password: prompt sekali per TUI session, hold di RAM saja —
  **tidak pernah** ditulis ke disk.
- CLI ambil dari `DBSYNC_MASTER_KEY` (32-byte hex / 64 char) → fallback
  stdin prompt → fail dengan instruction jelas kalau non-interactive.
- Log redaction: SQL template di-log, argumen tidak. Password field di
  error message: selalu `***`.

Detail di `docs/PRD-v1.md` §Security.

---

## 7. Workflow agent

**Wajib pakai `bd` (Beads) untuk task tracking** — bukan TodoWrite,
bukan markdown TODO list. Aturan ini ada di `CLAUDE.md`.

Per issue:

1. `bd ready` — ambil issue yang siap (open + unblocked).
2. `bd update <id> --claim` — claim.
3. Baca `docs/issues/00N-*.md` lengkap (= agent brief).
4. Baca `~/.claude/rules/context7.md`, lalu query `context7` untuk
   tiap library yang disebut di section "REQUIRED" issue body.
5. Implement sesuai langkah numbered di body issue.
6. Penuhi semua acceptance criteria.
7. `bd close <id>` setelah PR merged.
8. `bd ready` lagi — biasanya ada issue baru yang unblocked.

**Session close** (CLAUDE.md mandatory):
```bash
git pull --rebase
bd dolt push
git push
git status   # harus "up to date with origin"
```

---

## 8. Dependency graph (8 issues, v1)

```
001 ── 002 ── 003 ── 004 ── 005 ────────┐
 │                                       │
 └──────── 006 ── 007 ─────────── 008 ──┘
```

- **CLI track:** 002 → 003 → 004 → 005
- **TUI track:** 006 → 007 (butuh 003) → 008 (butuh 005 + 007)
- **Critical path:** 001 → 002 → 003 → 004 → 005 → 008

Sumber kebenaran dependency: **Beads** (`bd ready`). File markdown hanya
peta tingkat tinggi — bisa basi, Beads tidak.

Status snapshot (2026-05-26): 1 `ready-for-agent` (`dbsync-61b` / GH #1),
7 `needs-triage` (blocked sampai blocker close).

---

## 9. Penting saat coding

- **Sebelum edit symbol:** jalankan `gitnexus_impact({target, direction:"upstream"})`.
  Lihat `CLAUDE.md` §GitNexus.
- **Sebelum commit:** jalankan `gitnexus_detect_changes()`.
- **Sebelum coding library X:** query `context7` untuk dapat snippet
  versi terkini. Cek `~/.claude/rules/context7.md`.
- **Tidak pakai** `ls -R` / grep buta — pakai `gitnexus_query` /
  `mcp_graphify_*` (rules `graphify.md`).

---

## 10. Pointer cepat

| Mau tahu | Buka |
|----------|------|
| Apa yang dibangun & kenapa | `docs/PRD-v1.md` |
| Arsitektur & schema | `docs/ARCHITECTURE.md` |
| Urutan kerja & dependency | `docs/EXECUTION-ORDER.md` |
| Detail per issue (agent brief) | `docs/issues/00N-*.md` |
| Aturan agent (beads, session close) | `CLAUDE.md` |
| GitHub mirror | https://github.com/kentoespdam/dbsync/issues |

---

*Last updated: 2026-05-26.*
