# dbsync v1 — Execution Order

Urutan pengerjaan vertical-slice issues. Setiap baris adalah satu
tracer bullet yang sudah dispesifikasi penuh di `docs/issues/00N-*.md`.

Issue yang siap dikerjakan sekarang ditandai dengan **▶**. Lainnya
masih `blocked` sampai blocker-nya close.

> Cara baca: agent berikutnya wajib `bd ready` dulu — sumber kebenaran
> dependency adalah Beads, bukan file ini. File ini hanya peta jalan
> tingkat tinggi.

## Dependency graph

```
001 ── 002 ── 003 ── 004 ── 005 ────────┐
 │                                       │
 └──────── 006 ── 007 ─────────── 008 ──┘
```

## Order list

| S | Urut | Issue | Beads ID | GH | Slice | Track | Blocked by | State |
|---|------|-------|----------|----|----|-------|------------|-------|
| [x] | 1 | [001](issues/001-encrypted-credential-storage.md) | `dbsync-61b` | [#1](https://github.com/kentoespdam/dbsync/issues/1) | Encrypted credential storage | CLI | — | `closed` |
| [x] | 2 | [002](issues/002-mysql-connectivity-schema-inspection.md) | `dbsync-i39` | [#2](https://github.com/kentoespdam/dbsync/issues/2) | MySQL connectivity & schema inspection | CLI | 001 | `closed` |
| [x] | 3 | [003](issues/003-column-mapping-crud.md) | `dbsync-spj` | [#3](https://github.com/kentoespdam/dbsync/issues/3) | Column mapping CRUD | CLI | 002 | `closed` |
| [x] | 4 | [004](issues/004-single-batch-sync-tracer.md) | `dbsync-afe` | [#4](https://github.com/kentoespdam/dbsync/issues/4) | Single-batch sync tracer | CLI | 003 | `closed` |
| [x] | **5** | [005](issues/005-checkpoint-resume-history-all-tables-dry-run.md) | `dbsync-c6i` | [#5](https://github.com/kentoespdam/dbsync/issues/5) | Checkpoint, resume, history, `--all-tables`, `--dry-run` | CLI | 004 | `closed` |
| [x] | 6 | [006](issues/006-tui-shell-connection-management.md) | `dbsync-45p` | [#6](https://github.com/kentoespdam/dbsync/issues/6) | TUI shell + connection management | TUI | 001 | `closed` |
| [x] | 7 | [007](issues/007-tui-table-picker-mapping-editor.md) | `dbsync-nvt` | [#7](https://github.com/kentoespdam/dbsync/issues/7) | TUI table picker + mapping editor | TUI | 003, 006 | `closed` |
| [x] | 8 | [008](issues/008-tui-run-screen-history-viewer.md) | `dbsync-0mr` | [#8](https://github.com/kentoespdam/dbsync/issues/8) | TUI run / history / checkpoint screens | TUI | 005, 007 | `closed` |

## Jalur paralel

Setelah **001** selesai, dua jalur bisa berjalan **paralel**:

- **CLI track:** 002 → 003 → 004 → 005
- **TUI track:** 006 → 007 (butuh 003 juga) → 008 (butuh 005 + 007)

Titik konvergensi: **008** butuh dua-duanya selesai.

## Critical path

```
001 → 002 → 003 → 004 → 005 → 008
```

6 issue di critical path. 006 dan 007 tidak di critical path tapi
**harus** selesai sebelum 008 bisa start.

## Saran scheduling

- **1 agent (sequential):** ikuti kolom "Urut" 1→8. ~8 chunks of work.
- **2 agent (paralel setelah 001):**
  - Agent A: 002 → 003 → 004 → 005 (CLI)
  - Agent B (tunggu 003 close): 006 → 007 (TUI shell + editor)
  - Salah satu pegang 008 setelah 005 + 007 close.

## Workflow per issue

Untuk setiap issue, agent harus:

1. `bd ready` → ambil ID yang masih open + unblocked.
2. `bd update <id> --claim` → claim work.
3. Baca `docs/issues/00N-*.md` body penuh — body adalah agent brief.
4. Baca `~/.claude/rules/context7.md`, lalu query `context7` untuk
   semua library yang disebutkan di section "REQUIRED" issue body.
5. Implementasi sesuai langkah numbered di body issue.
6. Penuhi semua acceptance criteria (checkboxes di bagian bawah).
7. `bd close <id>` setelah PR merged.
8. Cek lagi `bd ready` — biasanya ada issue baru yang unblocked.

## Aturan kunci dari PRD

- **Simple > clever.** Tidak ada DI framework, tidak ada abstraksi layered.
- **SQLite = SSOT.** TUI dan CLI tidak boleh pegang state config in-memory
  yang divergen dari database.
- **Test priority:** `crypto`, `storage`, `mysql`, `engine` wajib ada test;
  `cli` dan `tui` cukup manual QA.
- **Integration test:** tag dengan `//go:build integration` supaya
  `go test ./...` default tidak butuh Docker.
- **PR description:** wajib sebutkan library mana saja yang sudah
  di-query lewat `context7`, plus rangkuman keputusan teknis kalau
  menyimpang dari issue body.

## Status snapshot

Update tanggal: **2026-05-26**

- 8 issues total, semua ter-mirror di Beads + GitHub.
- 0 ready-for-agent.
- 0 needs-triage.
- 0 in progress, 8 closed (all).

Untuk status terkini, jalankan:

```bash
bd ready          # issue yang siap diambil
bd list           # semua issue
bd show <id>      # detail satu issue
```
