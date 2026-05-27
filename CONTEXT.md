# dbsync — Project Context

> Quick orientation. Full spec: `docs/PRD-v1.md`, `docs/ARCHITECTURE.md`.

## What
Single-binary Go tool untuk **MySQL table sync satu arah** (source → dest).
Bikin operasi ad-hoc (`mysqldump`, shell script) jadi **auditable, resumable, cron-friendly**.

Dua mode, satu binary, satu SQLite:
- **TUI** `./dbsync` — setup koneksi, mapping, sync interaktif.
- **CLI** `./dbsync run --connection=X --table=Y` — cron. Exit 0/1/2.

Status: **v1, UI overhaul phase** (bd-09a/b complete).

## Why (5 pain points)
1. Tidak ada riwayat sync.
2. Tidak resumable.
3. Tidak ada column mapping (rename, default, kolom ekstra di dest).
4. Credential plaintext di script.
5. Tidak cocok untuk cron.

## Prinsip non-negotiable
- Simple > clever. No DI framework. Junior dev + local AI readable.
- **SQLite = SSoT.** TUI/CLI tidak boleh state divergen.
- **Frontend-agnostic core.** Engine emit `<-chan Event`.
- **No CGo** (`modernc.org/sqlite`).
- Test wajib: `crypto`, `storage`, `mysql`, `engine`. Manual QA: `cli`, `tui`.
- Integration test pakai `//go:build integration`.

## Layout
```
cmd/dbsync/         entry point
internal/
├── crypto/         AES-256-GCM + scrypt KDF  ✅
├── config/         master password lifecycle  ✅
├── storage/        SQLite repo + migration    ✅ (partial)
├── mysql/          pool + INFO_SCHEMA + upsert  ⟵ TBD
├── engine/         sync orchestrator            ⟵ TBD
├── logger/         JSONL log + redaction        ⟵ TBD
├── cli/            cobra handlers              ✅ skeleton
└── tui/            bubbletea                    ⟵ TBD
docs/{PRD-v1, ARCHITECTURE, EXECUTION-ORDER}.md + issues/001-008.md
```

## Stack
Go 1.25 · bubbletea+bubbles+lipgloss · cobra · go-sql-driver/mysql · modernc.org/sqlite · x/crypto/scrypt · x/term.

## Security (ringkas)
- AES-256-GCM, nonce 12-byte random prepended.
- scrypt KDF: N=32768, r=8, p=1, keyLen=32. Salt di `~/.config/dbsync/salt`.
- Master password: TUI prompt 1x/session, RAM only. CLI dari `DBSYNC_MASTER_KEY` (64-char hex) → stdin → fail jelas.
- Log redaction: SQL template di-log, argumen tidak. Password di error: `***`.

## Workflow agent
Pakai **bd (Beads)** untuk task tracking (bukan TodoWrite). Per issue:
1. `bd ready` → `bd update <id> --claim`
2. Baca `docs/issues/00N-*.md` + query `context7` untuk lib di section "REQUIRED".
3. Implement + penuhi acceptance criteria.
4. `bd close <id>`.

**Session close:** `git pull --rebase && bd dolt push && git push && git status`.

## Dependency graph (8 issues v1)
```
001 ── 002 ── 003 ── 004 ── 005 ────┐
 │                                   │
 └──── 006 ── 007 ──────────── 008 ──┘
```
CLI: 002→003→004→005. TUI: 006→007(butuh 003)→008(butuh 005+007).
Critical path: 001→002→003→004→005→008.
SSoT dependency = Beads (`bd ready`), bukan markdown.

Snapshot 2026-05-27: `bd-09` series completed. Issue 009 finalized.

## Coding rules
- Sebelum edit symbol: `gitnexus_impact({target, direction:"upstream"})`.
- Sebelum commit: `gitnexus_detect_changes()`.
- Sebelum pakai lib X: query `context7`.
- Jangan `ls -R` / grep buta — pakai `gitnexus_query` / `mcp_graphify_*`.

## Pointer
| Mau tahu | Buka |
|---|---|
| Apa & kenapa | `docs/PRD-v1.md` |
| Arsitektur & schema | `docs/ARCHITECTURE.md` |
| Urutan kerja | `docs/EXECUTION-ORDER.md` |
| Detail issue | `docs/issues/00N-*.md` |
| Aturan agent | `CLAUDE.md` |
| GitHub mirror | https://github.com/kentoespdam/dbsync/issues |

*Last updated: 2026-05-26.*
*
