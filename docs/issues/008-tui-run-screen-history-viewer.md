# Issue 008 — TUI run screen + history viewer + checkpoint viewer

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 005, Issue 007
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 12, 13, 20, 23

---

## What to build

Screen TUI untuk jalankan sync interaktif (progress bar, ETA, error
panel real-time), screen history list per tabel, dan screen
checkpoint viewer (untuk lihat sync yang sedang/sempat interrupted).

Ini slice terakhir — setelah ini v1 feature-complete.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/charmbracelet/bubbletea`** — topik: "subscribing to Go
   channel via tea.Cmd polling pattern, throttling Msg, graceful shutdown".
2. **`github.com/charmbracelet/bubbles`** — topik: "progress component,
   viewport for log tail, table component sortable".
3. **`github.com/charmbracelet/lipgloss`** — topik: "fixed-height panels,
   responsive 3-panel layout, color-coded status".

---

## Step-by-step implementation

### Step 1 — Run screen layout

File: `internal/tui/run_screen.go`

Layout (vertical stack):
```
┌─ Sync: prod-to-staging / users ─────────────────────┐
│ Batch 12 / ~50      [████████░░░░░░░░] 45%   ETA 2m │ <- progress
├─────────────────────────────────────────────────────┤
│ ↳ batch 1: 1000 rows upserted                       │
│ ↳ batch 2: 1000 rows upserted                       │
│ ↳ batch 3: 1000 rows upserted                       │ <- event log (viewport)
│ ⚠ batch 4: row pk=4521 — duplicate key 'email'     │
│ ↳ batch 5: rolled back                              │
├─────────────────────────────────────────────────────┤
│ [c] cancel  [p] pause (n/a v1)  [esc] back when done│
└─────────────────────────────────────────────────────┘
```

Components:
- `progress.Model` dari bubbles, percentage dihitung dari
  `(rowsDone / estimatedTotal)` × 100. Estimated total: query
  `SELECT COUNT(*)` di source (1x di awal, async).
- `viewport.Model` untuk event log (auto-scroll, scrollable manual saat
  user tekan `↑`).
- ETA: estimasi linear berdasarkan elapsed time dan progress.

### Step 2 — Engine channel subscription

Pattern bubbletea untuk subscribe ke `<-chan engine.Event`:

```go
type eventMsg engine.Event

func waitForEvent(ch <-chan engine.Event) tea.Cmd {
    return func() tea.Msg {
        ev, ok := <-ch
        if !ok { return eventChannelClosedMsg{} }
        return eventMsg(ev)
    }
}

// di Update: setelah handle eventMsg, schedule ulang waitForEvent
// supaya polling channel berkelanjutan.
```

Pre-flight di `Init` atau saat enter screen:
- Resolve checkpoint untuk (conn, table):
  - Status `interrupted` → modal prompt "previous run was interrupted
    at batch N (PK=...). [r] resume, [f] fresh start, [esc] cancel".
  - Status lain → fresh.
- Spawn count estimation `tea.Cmd` (async).
- Call `engine.Run(ctx, opts)` → dapat channel → `waitForEvent`.

### Step 3 — Event handling

- `ProgressEvent` → update batch count, rowsDone, progress bar,
  append "↳ batch N: M rows upserted" ke viewport.
- `BatchErrorEvent` / `RowErrorEvent` → append warning line (color
  `errorStyle`) ke viewport.
- `DoneEvent` → freeze progress, show summary:
  - Sukses: `✓ Done. 50000 rows in 4m12s.`
  - Failed: `✗ Failed at batch 4. See log: <jsonl path>.`
  - Interrupted (user cancel): `⚠ Interrupted. Resume from TUI or CLI.`
  - Tombol berubah: `[esc] back`, `[v] view log file path`.

### Step 4 — Cancellation

- `c` → tanya confirm "Cancel sync? Progress will be saved as
  checkpoint." → `cancelFunc()` (context).
- Engine harus respect ctx → mark checkpoint `interrupted`, emit
  `DoneEvent` dengan err `context.Canceled`.
- UI tunggu `DoneEvent` sebelum allow back navigation.

### Step 5 — Multi-table sync (all-tables)

Variant flow: kalau user pilih "Sync all mapped tables" di main menu:
- Sequential per tabel, sama dengan CLI `--all-tables`.
- Header tampilkan `[3/10] users` untuk progress per-tabel.
- Summary akhir: `8 success, 1 partial, 1 fatal` dengan list per tabel.

### Step 6 — History viewer screen

File: `internal/tui/history.go`

Layout:
- `table.Model` kolom: `STARTED | DURATION | TABLE | ROWS | STATUS | ERROR`.
- Filter by connection (header). Filter by table (`/`).
- Status colored: green=completed, red=failed, yellow=interrupted, blue=running.
- Pagination atau scroll dengan limit 100 row terbaru.
- `enter` → detail screen dengan field lengkap + path ke log file.

Data: `storage.HistoryRepo.List(ctx, connID, table, limit)`.

### Step 7 — Checkpoint viewer screen

File: `internal/tui/checkpoints.go`

- Table kolom: `CONNECTION | TABLE | STATUS | BATCH | UPDATED`.
- Hanya tampilkan status `running` (warning: stale) atau `interrupted`.
- Action:
  - `r` → resume di run screen (transition).
  - `x` → reset (delete checkpoint) dengan confirm.

Data: `storage.CheckpointRepo.ListActive(ctx)`.

### Step 8 — Wire main menu

Update main menu:
- Sync → pilih connection → pilih tabel → run screen (atau "all tables").
- History → pilih connection → history viewer.
- Checkpoints → checkpoint viewer langsung (cross-connection).

### Step 9 — Manual QA

- [ ] Run small sync (100 row): progress bar 0→100%, viewport log scroll.
- [ ] Run large sync (50k row): ETA stabil setelah 2-3 batch.
- [ ] Cancel mid-sync (c → confirm): checkpoint `interrupted`, exit smooth.
- [ ] Resume from interrupted: prompt muncul, [r] → mulai dari last PK.
- [ ] Fail scenario: hapus PK constraint di dest sementara → batch error
  muncul di viewport + JSONL log path tampil di summary.
- [ ] All-tables sync: header `[N/total]` benar, summary final akurat.
- [ ] History viewer: filter by connection/table, color status benar.
- [ ] Checkpoint viewer: reset action menghapus row.
- [ ] Resize terminal saat run: layout tidak rusak.

---

## Acceptance criteria

- [ ] Run screen layout (header / progress / viewport / footer) render benar.
- [ ] Subscribe ke `engine.Event` channel via tea.Cmd polling tanpa busy-loop.
- [ ] ProgressEvent update progress bar + viewport log line.
- [ ] BatchErrorEvent / RowErrorEvent ditampilkan warning style di viewport.
- [ ] DoneEvent freeze UI, summary akurat per status.
- [ ] Cancellation: `c` → confirm → ctx canceled → checkpoint interrupted.
- [ ] Resume prompt saat enter run screen kalau checkpoint interrupted ada.
- [ ] All-tables sync: header `[i/N]`, summary aggregated.
- [ ] History viewer: list + filter + color-coded status + detail screen.
- [ ] Checkpoint viewer: list active + reset action.
- [ ] Path ke JSONL log file ditampilkan saat sync gagal (untuk grep manual).
- [ ] Resize terminal saat run screen aktif: layout responsive.
- [ ] Manual QA checklist semua ✓.
- [ ] `go build` clean, `go vet` clean.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

## Blocked by

- Issue 005 (butuh engine full + checkpoint/history repos).
- Issue 007 (butuh table picker pattern + main menu wiring).
