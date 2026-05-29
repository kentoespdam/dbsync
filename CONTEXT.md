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
├── applog/         slog + lumberjack (app log)  ✅
├── logger/         JSONL log per-sync           ✅
├── paths/          binary-relative path helper  ✅
├── cli/            cobra handlers              ✅ skeleton
└── tui/            bubbletea                    ⟵ TBD
docs/{PRD-v1, ARCHITECTURE, EXECUTION-ORDER}.md + issues/001-008.md
```

## Stack
Go 1.25 · bubbletea+bubbles+lipgloss · cobra · go-sql-driver/mysql · modernc.org/sqlite · x/crypto/scrypt · x/term · Release: GoReleaser v2 + GitHub Actions.

## Release artifact
- **Binary:** Single file native (`dbsync` atau `dbsync.exe`), statis (`CGO_ENABLED=0`), strip symbols.
- **Versi:** Otomatis di-inject via `-ldflags` (`main.version`, `main.commit`, `main.date`) dari Git tag.
- **Distribusi:** `.tar.gz` (Linux) dan `.zip` (Windows) via GitHub Releases.
- **Keamanan:** SHA-256 checksums tersedia di setiap rilis.
- **Windows:** Tidak di-sign (Authenticode). User harus melewati SmartScreen warning via "Run anyway".

## Logging dua-jalur
- **`applog`** (slog → `<exeDir>/logs/dbsync.log`, rotated via lumberjack) untuk debug & error review aplikasi sehari-hari. Format text/logfmt, `AddSource:true`, level via `DBSYNC_LOG_LEVEL`. File-only writer (TUI haram stdout).
- **`logger`** (JSONL → `<exeDir>/logs/sync-<ts>-<conn>-<table>.jsonl`) tetap dipakai untuk error journal per-sync (row/batch error). Entry shape & filename **tidak berubah**; hanya direktori output yang pindah dari `~/.local/share/...` ke binary-relative.
- Error MySQL ditulis apa adanya di kedua jalur agar actionable (lihat [ADR 0003](docs/adr/0003-remove-mysql-error-redaction.md)). Password connection MySQL tetap di-redact (`***`) di `internal/mysql/pool.go redactError`.

## Security (ringkas)
- AES-256-GCM, nonce 12-byte random prepended.
- scrypt KDF: N=32768, r=8, p=1, keyLen=32. Salt di `~/.config/dbsync/salt`.
- Master password: TUI prompt 1x/session, RAM only. CLI dari `DBSYNC_MASTER_KEY` (64-char hex) → stdin → fail jelas.
- Log: SQL template di-log, argumen tidak. Password connection di error: `***` (lihat `internal/mysql/pool.go redactError`). Error MySQL lain (constraint violation, schema mismatch, dll) ditulis apa adanya untuk debuggability — lihat [ADR 0003](docs/adr/0003-remove-mysql-error-redaction.md).

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

## Glossary

- **Value Map** — translasi 1:1 antara domain value source dan dest untuk satu
  `(connection, table, dest_column)`. Disimpan sebagai JSON `{src:dest}` di
  `mappings.value_map` (TEXT, `CHECK json_valid`). Orthogonal dengan
  `default_value`: `default_value` dipakai saat row source NULL; `value_map`
  dipakai saat row source ada nilainya. Tanpa `value_map` → passthrough
  (backward-compat). Lookup miss → row di-log via `internal/logger` (JSONL),
  batch lanjut, sync exit code 1 — konsisten dgn filosofi "auditable" dan
  [ADR 0003](docs/adr/0003-remove-mysql-error-redaction.md). Validasi storage:
  kalau dest kolom ENUM, semua *value* map harus ∈ `Column.EnumValues(dest)`;
  kalau dest bukan ENUM, no check.
- **Test Connection** — dua jalur, semantik beda:
  1. *Form test* (`connFormModel.testSource/testDest` di `internal/tui/conn_form.go`) — validasi credential yang user **sedang ketik** di form, dipicu saat tekan Enter di field terakhir. Sukses → langsung lanjut save. Gagal → prompt "Save anyway? (y/N)". Tidak menampilkan layar hasil.
  2. *List test* (`connTestModel.testAll` di `internal/tui/conn_check.go`) — re-test koneksi **yang sudah tersimpan**, password didekripsi dari SQLite. Dipicu dari list connection dengan tombol `t`. Tampilkan status `✓ OK` / `✗ <error>` untuk Source dan Dest, lalu tekan key apa pun untuk balik. Error MySQL ditampilkan apa adanya (password connection sudah di-redact di `mysql/pool.go`).
  > Aturan Bubble Tea: `tea.Cmd` jalan di goroutine — **jangan mutate model di Cmd**. Carry hasil di message payload, lalu `Update` yang menugaskan ke field model.

## Pointer
| Mau tahu | Buka |
|---|---|
| Apa & kenapa | `docs/PRD-v1.md` |
| Arsitektur & schema | `docs/ARCHITECTURE.md` |
| Urutan kerja | `docs/EXECUTION-ORDER.md` |
| Detail issue | `docs/issues/00N-*.md` |
| Distribusi & rilis | `docs/adr/0002-cross-platform-binary-distribution.md` |
| Kenapa error MySQL tidak di-redact | `docs/adr/0003-remove-mysql-error-redaction.md` |
| Kenapa pakai Value Map untuk translasi ENUM | `docs/adr/0005-value-map-for-enum-translation.md` |
| Aturan agent | `CLAUDE.md` |
| GitHub mirror | https://github.com/kentoespdam/dbsync/issues |

*Last updated: 2026-05-29.*
*
