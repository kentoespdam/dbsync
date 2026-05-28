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

### bd-09e — Table picker UX (Flow A: Tables & Mappings)

**Konteks:** Setelah `Connection → Tables & Mappings`, user mendarat di
`tablePickerModel` (Title `Select Table: <conn>`). Flat list bubbles
default tanpa filter visible + badge teks `[mapped]`/`[no-pk]` di
description. Saat tabel puluhan, scroll manual menyebalkan dan status
mapped tidak ke-spot.

File: `internal/tui/table_picker.go` (extend, bukan rewrite).

#### Layout

```
┌────────────────────────────────────────────────────────────────┐
│ Tables: conn-prod                                              │   ← title
│ 40 tables • 12 mapped • 28 unmapped • 2 no-pk                  │   ← stats line 1 (absolut)
│ Filter: 'user' • unmapped-only • showing 3                     │   ← stats line 2 (opsional)
├────────────────────────────────────────────────────────────────┤
│ ┌── filter ────────────────────────────────────────────────┐   │
│ │ user_                                                    │   │   ← textinput, border pink saat focus
│ └──────────────────────────────────────────────────────────┘   │
├────────────────────────────────────────────────────────────────┤
│ ✓  users                                                       │
│ ○  user_logs               [no-pk]                             │   ← cursor here (list focused)
│ ○  user_sessions                                               │
├────────────────────────────────────────────────────────────────┤
│ tab focus • ↑↓ nav • enter open • u unmapped-only • r reload  │   ← static help bar
│ esc back                                                       │
└────────────────────────────────────────────────────────────────┘
```

#### Status ikon (2 state + badge)

| Ikon | Arti | Kondisi |
|------|------|---------|
| `✓` (hijau 10) | sudah punya mapping di DB | `Mappings().Exists(connID, table) == true` |
| `○` (abu 240) | belum di-map | `!Exists` |

`[no-pk]` tetap badge teks di description (kuning 208), tidak block masuk
mapping editor (sync engine yang reject; mapping editor scope kolom saja).

#### Always-on filter input

- `textinput.Model` selalu visible di atas list, default focus.
- Border lipgloss: pink `170` saat focused, abu `240` saat blur.
- Tab switches focus filter ↔ list.
- Saat filter focused: ketikan masuk filter text; arrow/enter/u/r tidak
  intercept (full ownership ke input). Tab pindah ke list.
- Saat list focused: arrow nav, enter select, `u` toggle, `r` reload,
  `esc` back. Tab balik ke filter.

#### Filter compose (AND)

- Filter teks = substring match terhadap `tableItem.name` (case-insensitive).
- Toggle `u` = filter ke ikon `○` saja.
- Aktif bersamaan = AND. Stats line 2 muncul format:
  `Filter: 'user' • unmapped-only • showing 3`.
- Kalau cuma toggle aktif: `Filter: unmapped-only • showing 28`.
- Kalau cuma teks: `Filter: 'user' • showing 5`.
- Tidak ada filter aktif → stats line 2 tidak muncul.

#### Sort

- Alphabetic ascending (mengikuti `mysql.ListTables` natural order).
- Tidak ada grouping mapped/unmapped (sort posisi stabil setelah edit
  mapping → kembali ke screen ini, tabel di posisi yang sama).

#### Keybinding ringkasan

| Key | Konteks | Aksi |
|-----|---------|------|
| `tab` | global | switch focus filter ↔ list |
| `enter` | filter focus | (no-op; teks tetap di input) |
| `enter` | list focus | open table (masuk mapping editor) |
| `↑/↓` | list focus | nav |
| `u` | list focus | toggle unmapped-only |
| `r` | list focus | reload (re-query source DB + re-check Exists) |
| `esc` | global | back to connection picker |

#### Implementasi

- Extend `tablePickerModel`: tambah `filterInput textinput.Model`,
  `unmappedOnly bool`, `focused int` (0=filter, 1=list).
- `View()` pecah jadi 4 segmen: stats, filter input (bordered), list,
  help bar. Pakai `lipgloss.JoinVertical`.
- `Update()`:
  - `tab` → toggle `focused`, re-set border filter input style.
  - Saat `focused == 0`: route msg ke `filterInput.Update`, lalu
    `applyFilter()`.
  - Saat `focused == 1`: tangani `u`, `r`, `enter`, default forward ke
    `list.Update`.
- `applyFilter()`:
  - Iterasi `allItems`, filter by substring AND by unmapped flag.
  - `list.SetItems(filtered)`, hitung stats untuk header.
- Store `allItems` (full set) terpisah dari `list.Items()` (filtered).
- **Penting:** Stats line 1 selalu hitung atas `allItems`, BUKAN atas
  `list.Items()` (yang sudah filtered).
- Custom delegate `tableItemDelegate` render prefix ikon dengan warna
  via `lipgloss.Style`. Badge `[no-pk]` tetap di description (default
  delegate sudah hide kalau pakai custom).

#### Out of scope (bd-09e)

- Bulk select & bulk-map (defer).
- "Run all unmapped" shortcut dari screen ini (defer).
- Sort by status / by column count (defer).
- Live polling refresh (cukup manual `r`).

Acceptance:
- [ ] Filter input visible saat masuk screen, default focused (border pink).
- [ ] Ketik teks di filter → list mengecil sesuai substring; stats line 2 muncul.
- [ ] Tab → focus pindah list (cursor highlight); border filter jadi abu.
- [ ] `u` di list focused → toggle unmapped-only; stats line 2 update.
- [ ] Filter teks + `u` aktif → AND compose; stats `Filter: 'X' • unmapped-only • showing N`.
- [ ] Tabel mapped tampil ikon `✓` hijau; unmapped tampil `○` abu.
- [ ] `[no-pk]` tetap visible di description tabel tanpa PK; tidak block `enter`.
- [ ] `r` reload re-query + re-check Exists; filter state TIDAK di-reset.
- [ ] Stats line 1 (`40 tables • 12 mapped • ...`) selalu menampilkan total absolut, tidak ikut filter.
- [ ] Reload setelah edit mapping → tabel yang baru di-map ikon berubah `○` → `✓`.

#### Test

- Manual QA: utamakan flow A. Flow B (`Select Table to Sync`) tetap pakai
  model yang sama; verifikasi tidak regresi (toggle `u` di flow B harus
  tetap jalan tapi `--- SYNC ALL MAPPED TABLES ---` item tetap di top).
- Tidak ada unit test wajib (TUI scope per `CLAUDE.md`).

---

## Dependency chain

```
bd-09a (schema+automap) ──► bd-09b (table redesign) ──► bd-09c (modal+validation) ──► bd-09d (toast) ──► bd-09e (table picker UX)
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
- [ ] Table picker (Flow A): always-on filter, ikon ✓/○, stats 2-line, toggle `u` unmapped-only, tab switch focus.

---

## Out of scope (defer)

- Bulk edit (select multiple rows + apply same default).
- Mapping templates (save/load preset).
- Diff view (show what changed vs DB).
- Undo/redo dalam editor.
- Custom transform expression (selain default value).

Kalau dibutuhkan nanti, file issue baru.
