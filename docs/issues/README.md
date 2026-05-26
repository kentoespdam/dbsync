# dbsync v1 — Issues

Vertical-slice issues yang turunan dari [`../PRD-v1.md`](../PRD-v1.md).
Setiap issue adalah tracer bullet end-to-end (schema → backend → CLI/TUI
→ tests). Dependency dibuat eksplisit di field "Blocked by".

**Semua issue:** type `AFK` (agent-friendly, tidak butuh keputusan
arsitektur manusia di tengah). Semua di-label `needs-triage`.

## Issue tracker

- **Beads** (lokal, single source of truth): jalankan `bd ready` untuk
  lihat issue yang siap dikerjakan, `bd show <id>` untuk detail.
- **GitHub Issues** (mirror publik): https://github.com/kentoespdam/dbsync/issues

## Eksekusi order

```
001 ── 002 ── 003 ── 004 ── 005 ────────┐
 │                                       │
 └──────── 006 ── 007 ─────────── 008 ──┘
```

| # | Title | Beads ID | GH | Blocked by | Stories |
|---|---|---|---|---|---|
| [001](001-encrypted-credential-storage.md) | Encrypted credential storage | dbsync-61b | [#1](https://github.com/kentoespdam/dbsync/issues/1) | — | 1, 2, 4, 6, 24, 25 |
| [002](002-mysql-connectivity-schema-inspection.md) | MySQL connectivity & schema inspection | dbsync-i39 | [#2](https://github.com/kentoespdam/dbsync/issues/2) | 001 | 3, 7 |
| [003](003-column-mapping-crud.md) | Column mapping CRUD | dbsync-spj | [#3](https://github.com/kentoespdam/dbsync/issues/3) | 002 | 8, 9, 10, 11 |
| [004](004-single-batch-sync-tracer.md) | Single-batch sync tracer | dbsync-afe | [#4](https://github.com/kentoespdam/dbsync/issues/4) | 003 | 14, 15p, 21, 22 |
| [005](005-checkpoint-resume-history-all-tables-dry-run.md) | Checkpoint, resume, history, `--all-tables`, `--dry-run` | dbsync-c6i | [#5](https://github.com/kentoespdam/dbsync/issues/5) | 004 | 15, 16, 17, 18, 19, 20d, 23d |
| [006](006-tui-shell-connection-management.md) | TUI shell + connection management | dbsync-45p | [#6](https://github.com/kentoespdam/dbsync/issues/6) | 001 | 1, 3, 4, 5, 6 |
| [007](007-tui-table-picker-mapping-editor.md) | TUI table picker + mapping editor | dbsync-nvt | [#7](https://github.com/kentoespdam/dbsync/issues/7) | 003, 006 | 7, 8, 9, 10, 11 |
| [008](008-tui-run-screen-history-viewer.md) | TUI run / history / checkpoint screens | dbsync-0mr | [#8](https://github.com/kentoespdam/dbsync/issues/8) | 005, 007 | 12, 13, 20, 23 |

Total: 8 issues, semua user story dari PRD ter-cover.

## Instruksi umum untuk agent

1. **WAJIB** baca dan ikuti rule di `~/.claude/rules/context7.md` SEBELUM
   coding setiap issue. Setiap issue body menyebutkan library mana yang
   harus di-query.
2. Patuhi prinsip simple > clever (PRD §Implementation Decisions). Tidak
   ada DI framework, tidak ada abstraksi layered yang tidak perlu.
3. SQLite (`modernc.org/sqlite`) adalah single source of truth. TUI dan
   CLI tidak boleh pegang state config in-memory yang divergen dari DB.
4. Test priority sesuai PRD: `crypto`, `storage`, `mysql`, `engine`
   harus punya test; `cli` dan `tui` cukup manual QA.
5. Tag integration test dengan `//go:build integration` supaya
   `go test ./...` default tidak butuh Docker.
6. PR description: cantumkan **screenshot** output `context7` query yang
   dipakai, dan rangkuman keputusan teknis kalau menyimpang dari issue body.
