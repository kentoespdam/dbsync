# Issue 009 — Mapping editor UX overhaul

**Type:** AFK
**Triage label:** `needs-triage`
**Blocked by:** Issue 007 (mapping editor base sudah ada)
**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md), [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md)
**Related:** Issue 003 (`storage.AutoMap`), Issue 006 (TUI shell)

---

## Latar belakang

Mapping editor saat ini (`internal/tui/mapping_editor.go`) punya beberapa
gap UX dan satu bug substansial:

1. **Bug**: kolom dest yang tidak punya match di source TIDAK muncul di
   tampilan. `storage.AutoMap` hanya append ke `Mappings` kalau ada
   match — kolom seperti `isDeleted` (dest-only) hilang dari UI dan
   user tidak bisa set default-nya. Hanya muncul di warning panel.
2. **Warning panel statis**: warning ditampilkan di kanan, user harus
   pindah cursor manual ke baris bermasalah.
3. **Tombol `n` (add) tidak punya implementasi** — flag di-set tapi
   tidak ada form add manual.
4. **Default value editor generic** — TINYINT(1) (bool) dan ENUM tidak
   punya widget khusus, user harus tahu encoding yang dipakai engine.
5. **Save feedback bisu** — `successMsg` di `app.go:167` pop screen
   tanpa toast/konfirmasi.
6. **Validasi save lemah** — bisa save mapping dengan dest NOT NULL
   tanpa source maupun default; baru ketahuan saat sync gagal.
7. **Help bar pendek** — semua keybinding di-stack di satu baris,
   sulit dibaca dengan ~7 aksi.

Issue ini mendesain ulang mapping editor agar:
- Semua kolom dest selalu terlihat (sumber kebenaran adalah skema dest).
- Status setiap row jelas dari ikon, tanpa harus baca panel kanan.
- Default value type-aware untuk bool/enum.
- Save di-block kalau ada NOT NULL yang belum ter-resolve.
- Feedback save jelas (toast + auto-back).

---

## REQUIRED: Use `context7` BEFORE writing code

Patuhi `~/.claude/rules/context7.md`. Query wajib:

1. **`github.com/charmbracelet/bubbletea`** — topik: "modal overlay
   pattern, multiple child models, message routing to overlay vs base".
2. **`github.com/charmbracelet/bubbles/list`** — topik: "filtered list
   dengan custom delegate, set items dynamically".
3. **`github.com/charmbracelet/bubbles/table`** — topik: "table with
   custom row styling per row, dynamic column width".
4. **`github.com/charmbracelet/lipgloss`** — topik: "Place / overlay
   centered modal di atas base view, border + padding".

---

## Desain final (locked dari grilling session)

### Layout

```
┌────────────────────────────────────────────────────────────────┐
│ Mapping: users                                                 │   ← 3-line header block
│ src.app_users  →  dst.users                                    │
│ 12 cols • 10 mapped • 1 default • 1 ⚠ unresolved               │
├────────────────────────────────────────────────────────────────┤
│   ST  DEST COLUMN       SOURCE COLUMN     DEFAULT              │
│   ✓   id                id                -                    │
│   ✓   name              full_name         -                    │
│   ✓   email             email_address     -                    │
│   ●   synced_at         -                 NOW()                │
│   ●   tenant_id         -                 42                   │
│ ▶ ⚠   isDeleted         -                 -                    │   ← cursor here
│   ○   nickname          -                 -                    │
├────────────────────────────────────────────────────────────────┤
│ Selected: isDeleted                                            │
│ Type: TINYINT(1) NOT NULL  •  Status: needs source or default  │
├────────────────────────────────────────────────────────────────┤
│ e edit  n add-extra-dest  d delete  /  filter  w warnings-only │   ← 2-line context help
│ N next-warning   r reset   s save   esc back                   │
└────────────────────────────────────────────────────────────────┘
```

### Status ikon (4 state)

| Ikon | Arti | Kondisi |
|------|------|---------|
| `✓` | mapped via source | `SourceColumn.Valid == true` |
| `●` | default-only | `!SourceColumn.Valid && DefaultValue.Valid` |
| `⚠` | broken (NOT NULL, no source, no default) | dest `NOT NULL` & keduanya kosong |
| `○` | skipped (nullable, no source, no default) | dest nullable & keduanya kosong |

Warna (lipgloss): `✓` hijau, `●` cyan, `⚠` orange 208, `○` abu-abu 240.

### Synthetic rows

Semua kolom dest **selalu** muncul sebagai row. Kalau tidak ada record
di `sync_column_mappings`, buat synthetic `Mapping` in-memory:

```go
Mapping{
    ConnectionID: connID,
    TableName:    table,
    DestColumn:   dc.Name,
    SourceColumn: sql.NullString{Valid: false},
    DefaultValue: sql.NullString{Valid: false},
}
```

Synthetic row TIDAK di-insert ke DB pada save kecuali user mengisi
source atau default (yaitu mapping jadi valid).

### Edit modal

`e` membuka modal overlay (lipgloss `Place` di tengah). Field:

```
┌─ Edit mapping: isDeleted ──────────────┐
│ Type: TINYINT(1) NOT NULL              │
│                                        │
│ Source column:  [ filter... ]          │
│   (none)                               │
│   id                                   │
│   is_deleted          ← suggest top    │
│   deleted_at                           │
│                                        │
│ Default value:                         │
│   ( ) true                             │   ← bool widget
│   (•) false                            │
│                                        │
│ enter save • esc cancel                │
└────────────────────────────────────────┘
```

Source picker: searchable list, auto-suggest by Levenshtein/prefix
match terhadap `DestColumn`.

Default widget bersifat type-aware:
- `IsBool()` (TINYINT(1)): radio `true` / `false` / `(empty)`.
- `EnumValues()` non-empty (ENUM('a','b')): dropdown dari nilai valid.
- Lainnya: plain `textinput` dengan helper hint:
  `NOW(), CURRENT_TIMESTAMP, integer, "string"`.

### Save validation (hard-block)

Sebelum bulk-insert, scan synthetic + real mappings:
- Untuk tiap dest `NOT NULL` dengan `!SourceColumn.Valid && !DefaultValue.Valid`
  → tolak save, tampilkan toast merah:
  `Cannot save: 2 NOT NULL columns unresolved (isDeleted, status)`.
- Synthetic rows yang tidak valid (source & default keduanya kosong)
  tidak ikut di-insert (sesuai constraint `BulkInsert` yang existing).

### Filter & navigasi

- `/` → masuk filter mode (filter by DestColumn name).
- `w` → toggle "warnings-only" view (filter ke ikon `⚠` saja).
- `N` → loncat cursor ke `⚠` berikutnya (wrap around).

### Discard guard

Saat `esc` dengan `dirty == true`: confirm modal "Discard unsaved
changes? (y/N)" — pakai `showDiscardConfirm` pattern yang sudah ada di
`app.go:124-140`.

### Save feedback (toast)

Saat `successMsg` diterima `app.go`:
- Tampilkan toast "✓ Saved 12 mappings" di footer base view, auto-clear
  3 detik via `tea.Tick`.
- Setelah toast muncul, pop history (back ke table picker).

`successMsg` di-extend dengan field `message string`.

---

## Step-by-step implementation (4 issues sequential)

Pecah jadi 4 `bd` issues. Setiap issue self-contained, bisa di-merge
independen.

### bd-09a — Schema + AutoMap foundation

File:
- `internal/mysql/schema.go`: extend `Column` dengan `ColumnType string`
  (raw `COLUMN_TYPE` dari INFORMATION_SCHEMA). Tambah helper:
  ```go
  func (c Column) IsBool() bool { return strings.EqualFold(c.ColumnType, "tinyint(1)") }
  func (c Column) EnumValues() []string { /* parse "enum('a','b')" */ }
  ```
- `DescribeColumns`: query `COLUMN_TYPE` tambahan.
- `internal/storage/mappings.go`: ubah `AutoMap` agar SELALU append
  semua dest cols (synthetic untuk yang tidak ada match).
- Test: `mysql/schema_test.go` (bool/enum helpers), `storage/mappings_test.go`
  (synthetic rows hadir untuk dest-only cols).

Acceptance:
- [ ] `Column.IsBool()` true untuk `tinyint(1)`, false untuk `tinyint(4)`.
- [ ] `Column.EnumValues()` parse `enum('a','b','c')` → `["a","b","c"]`.
- [ ] `AutoMap` untuk dest cols `[id, isDeleted]` source `[id]` →
  `Mappings` punya 2 entry, `isDeleted` synthetic (source invalid).

### bd-09b — Table redesign (status icon + header + nav)

File: `internal/tui/mapping_editor.go`

- Tambah kolom `ST` (status icon) di table, width 4.
- Implement `mappingStatus(mp, dc)` returning ikon + warna.
- `refreshTable`: render ikon dengan lipgloss per row.
- Header: 3-line block (title, src→dst, stats).
- Help bar: 2-line, dipotong per grup logis.
- Keybindings: `/` (filter), `w` (warnings-only), `N` (next warning).
- Discard guard di `esc` (reuse `showDiscardConfirm`).
- `recomputeWarnings`: kerja atas synthetic rows juga.

Acceptance:
- [ ] Tabel `users` dengan dest col `isDeleted` tanpa source → row
      muncul dengan ikon `⚠`.
- [ ] `/` masuk filter mode, ESC keluar.
- [ ] `w` toggle filter ke ⚠ saja.
- [ ] `N` skip cursor ke ⚠ berikutnya.
- [ ] Stats di header live-update setelah edit.

### bd-09c — Modal edit (type-aware default)

File baru: `internal/tui/mapping_edit_form.go` (rewrite existing).

- Modal overlay via `lipgloss.Place(width, height, Center, Center, ...)`.
- Source field: `list.Model` dengan filter + `(none)` di top + auto-suggest
  (Levenshtein) berdasarkan dest column name.
- Default widget switch:
  - `dc.IsBool()` → radio component (kustom, 3 state).
  - `len(dc.EnumValues()) > 0` → list pick.
  - Else → `textinput` + helper text.
- Validation di modal: kalau dest NOT NULL & source = (none) → default
  required (inline error).
- Save validation (hard-block) di parent `mappingEditorModel.save()`:
  scan synthetic rows, kalau ada `⚠` → return error toast.

Acceptance:
- [ ] Edit `isDeleted` (TINYINT(1)) → modal show radio true/false.
- [ ] Edit kolom ENUM → modal show dropdown nilai valid.
- [ ] Save dengan 1 kolom `⚠` → toast merah, save di-block.
- [ ] Modal close on `esc`, parent state tidak ter-update.

### bd-09d — Save feedback (toast)

File: `internal/tui/app.go`, `internal/tui/toast.go` (baru).

- Extend `successMsg struct { message string }`.
- Tambah `toast` state di `model`: `toastMsg string`, `toastUntil time.Time`.
- `View`: kalau `time.Now().Before(toastUntil)`, overlay toast di footer
  base view (lipgloss border + bg).
- `successMsg` handler: set `toastMsg`, schedule `tea.Tick(3*time.Second, clearToastMsg{})`.
- `clearToastMsg` handler: reset `toastMsg`.

Acceptance:
- [ ] Save sukses → toast "✓ Saved N mappings" muncul 3 detik.
- [ ] Toast tidak block input (user bisa navigate selama toast tampil).
- [ ] Setelah pop history, toast tetap visible di screen sebelumnya.

---

## Dependency chain

```
bd-09a (schema+automap) ──► bd-09b (table redesign) ──► bd-09c (modal+validation) ──► bd-09d (toast)
```

Setiap PR di-merge berurutan. bd-09a tidak ubah TUI sama sekali (pure
storage + mysql), aman di-review independen.

---

## Manual QA (post bd-09d)

- [ ] Tabel `users` dengan dest col `isDeleted` (TINYINT(1) NOT NULL) →
      muncul sebagai row `⚠`.
- [ ] Edit `isDeleted` → set default `false` → save → toast → reload →
      mapping persist, ikon jadi `●`.
- [ ] Coba save dengan `⚠` masih ada → toast merah, mapping tidak ter-save.
- [ ] `/` filter cari `tenant_id` → row muncul, esc keluar filter.
- [ ] `w` mode warnings-only → hanya row `⚠` tampil.
- [ ] `N` di tengah list → cursor loncat ke `⚠` terdekat.
- [ ] Esc dengan dirty → confirm modal "Discard?".
- [ ] Edit kolom ENUM (`status enum('a','b','c')`) → modal show dropdown.
- [ ] Resize terminal → layout tetap usable.

---

## Acceptance criteria (rollup)

- [ ] Semua dest cols muncul sebagai row (bug `isDeleted` fixed).
- [ ] Status ikon 4-state benar untuk setiap row.
- [ ] Header 3-line dengan stats live-update.
- [ ] `/`, `w`, `N` navigasi berfungsi.
- [ ] Modal edit overlay center, type-aware default widget.
- [ ] Save hard-block kalau ada NOT NULL `⚠`.
- [ ] Toast "✓ Saved N mappings" auto-clear 3 detik.
- [ ] Discard guard saat esc dengan unsaved changes.
- [ ] Unit test untuk `IsBool`, `EnumValues`, synthetic `AutoMap`.
- [ ] `context7` MCP di-query SEBELUM coding (catat di PR description).

---

## Out of scope (defer)

- Bulk edit (select multiple rows + apply same default).
- Mapping templates (save/load preset).
- Diff view (show what changed vs DB).
- Undo/redo dalam editor.
- Custom transform expression (selain default value).

Kalau dibutuhkan nanti, file issue baru.
