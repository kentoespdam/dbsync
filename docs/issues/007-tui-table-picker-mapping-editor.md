# Issue 007 — TUI table picker + mapping editor

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 003, Issue 006
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md)
**User stories covered:** 7, 8, 9, 10, 11

---

## What to build

Screen TUI untuk browse tabel di source, generate auto-mapping, edit
mapping (rename kolom dest, set default value), dan warning panel untuk
kolom dest NOT NULL tanpa mapping.

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi rules di `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/charmbracelet/bubbletea`** — topik: "spawning long-running
   tea.Cmd (database query), result via custom Msg, cancellation".
2. **`github.com/charmbracelet/bubbles`** — topik: "table component with
   editable cells, list filter, viewport scroll".
3. **`github.com/charmbracelet/lipgloss`** — topik: "side-by-side layout
   with `lipgloss.JoinHorizontal`, dynamic width sizing".

---

## Step-by-step implementation

### Step 1 — Table picker screen

File: `internal/tui/table_picker.go`

Flow:
- Saat enter screen: spawn `tea.Cmd` untuk `mysql.ListTables` di source.
- Show spinner "Loading tables...".
- Result msg → populate `list.Model` dengan filter (typeable search).
- Untuk tiap row, tampilkan badge:
  - `[mapped]` kalau `MappingRepo.Exists(connID, table)`.
  - `[no-pk]` kalau `DetectPK` return empty (cache hasil).
- Key bindings:
  - `enter` → buka mapping editor untuk tabel terpilih.
  - `/` → filter mode.
  - `esc` → kembali.

### Step 2 — Mapping editor screen

File: `internal/tui/mapping_editor.go`

Layout 2 panel (lipgloss horizontal join):
- **Left panel:** list kolom dest dengan mapping saat ini:
  ```
  DEST COLUMN          SOURCE COLUMN     DEFAULT
  id                   id                -
  name                 name              -
  email                email_address     -
  synced_at            -                 NOW()
  tenant_id            -                 42
  ```
- **Right panel:** detail kolom yang dipilih + warnings.

Entry flow:
- Saat enter screen: spawn 2 `tea.Cmd` parallel:
  - `mysql.DescribeColumns` di source untuk tabel.
  - `mysql.DescribeColumns` di dest untuk tabel.
- Saat keduanya selesai:
  - Load mapping existing dari `MappingRepo.ListByTable`.
  - Kalau mapping kosong: panggil `storage.AutoMap(...)` untuk
    pre-fill (TIDAK insert ke DB sampai user tekan save).
- Tampilkan warning panel di kanan atau footer:
  ```
  ⚠ tenant_id (INT NOT NULL): no source mapping, no default — sync akan fail.
  ```

Key bindings:
- `↑/↓` → pilih row.
- `e` → edit row terpilih (modal/inline form):
  - Field: source column (dropdown dari source columns + "(none)"),
    default value (textinput).
- `n` → add row baru (untuk dest column ekstra yang tidak ada di source).
- `d` → delete row.
- `s` → save semua perubahan (bulk upsert ke DB).
- `r` → reset to auto-mapped.
- `esc` → confirm discard kalau ada unsaved changes, lalu back.

### Step 3 — Inline edit form (modal)

File: `internal/tui/mapping_edit_form.go`

- Modal overlay (lipgloss border + center positioning).
- Field:
  - Source column: list pilihan dari source columns, plus "(none / use default)".
  - Default value: textinput. Kalau source = "(none)", default wajib.
- Validasi: minimal salah satu source atau default harus ada.
- `enter` → apply ke parent model (belum commit ke DB).
- `esc` → cancel.

### Step 4 — Warning recalculation

Setiap kali mapping berubah (edit/add/delete), recompute warnings:
- Untuk tiap dest column `NOT NULL` tanpa default DB-level dan tanpa
  mapping atau dengan source = NULL + default = NULL → warning.
- Tampilkan count di header: `⚠ 2 warnings` (clickable / `w` untuk
  expand panel).

### Step 5 — Save flow

Saat `s`:
- Diff dengan mapping yang sudah ada di DB:
  - `Upsert` row yang berubah.
  - `Delete` row yang dihapus.
- Show spinner + success toast.
- Refresh dari DB (jaga konsistensi sesuai PRD: SQLite = SSOT).

### Step 6 — Connect ke main menu

Update main menu (Issue 006) → "Tables & Mappings":
- Pertama tanya pilih connection (kalau >1).
- Lalu transition ke table picker untuk connection itu.

### Step 7 — Manual QA

- [ ] Browse tabel: search, badge mapped/no-pk tampil benar.
- [ ] Open tabel tanpa mapping: auto-fill 1:1 pre-fill di UI (belum di-save).
- [ ] Save (s) → reload → mapping persist di DB.
- [ ] Edit mapping: rename source column → save → verify di DB.
- [ ] Set default value `NOW()` untuk kolom dest tambahan → save → sync (di Issue 008) berhasil.
- [ ] Warning panel update real-time saat edit.
- [ ] Discard prompt saat esc dengan unsaved changes.

---

## Acceptance criteria

- [ ] Table picker load tabel dari source MySQL via tea.Cmd async (tidak block UI).
- [ ] Badge `[mapped]` dan `[no-pk]` benar.
- [ ] Mapping editor split panel render benar, scroll independen.
- [ ] Auto-map pre-fill 1:1 untuk kolom dengan nama sama.
- [ ] Edit/add/delete mapping row interaktif, validasi source-or-default.
- [ ] Warning panel update setiap perubahan; count benar.
- [ ] Save bulk-upsert ke DB; diff-based (delete row yang dihapus).
- [ ] Reset to auto-mapped revert UI (belum touch DB sampai save).
- [ ] Discard prompt saat esc dengan unsaved changes.
- [ ] Resize terminal: layout tetap usable.
- [ ] Manual QA checklist semua ✓.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

## Blocked by

- Issue 003 (butuh MappingRepo + AutoMap helper).
- Issue 006 (butuh TUI shell + connection picker pattern).
