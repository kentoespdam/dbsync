# Issue 009 — Claim order & progress checklist

**Parent plan:** [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md)
**Tujuan:** Pastikan 4 sub-issue dikerjakan **berurutan**, satu per satu, tanpa overlap. Tiap agent claim ID berikutnya HANYA setelah PR sebelumnya merged ke `main`.

---

## ⚠️ Aturan main (baca dulu)

1. **Plan doc adalah sumber kebenaran.** Buka [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md) **sebelum** menulis kode. Jangan improvisasi di luar yang sudah locked di sana.
2. **Patuh dependency chain** — jangan claim issue yang dependency-nya belum closed.
3. **Satu agent = satu issue.** Jangan paralel di branch yang sama.
4. **Patuhi `CLAUDE.md`** project rules: DRY comments, max 120 baris per file/function, `context7` wajib untuk lib eksternal, test wajib untuk `mysql`/`storage`.
5. **Workflow per issue:**
   ```
   bd update <bd-id> --claim
   git checkout -b feat/<bd-id>-<slug>
   # … kerja, commit kecil-kecil …
   go test ./...
   go build -o dbsync ./cmd/dbsync
   git push -u origin feat/<bd-id>-<slug>
   gh pr create --base main --title "<bd-id>: …" --body "Closes #<gh-num>. Plan: docs/issues/009-mapping-editor-ux.md"
   # … review + merge …
   bd close <bd-id>
   ```
6. **Session close MANDATORY** (lihat `CLAUDE.md` "Session Completion"): `git push` + `bd dolt push`.

---

## Claim order (linear chain)

```
bd-09a (foundation)  ──►  bd-09b (table redesign)  ──►  bd-09c (modal + validation)  ──►  bd-09d (toast)  ──►  bd-09e (table picker UX)
   #9                       #10                          #11                              #12                  (gh TBD)
```

---

## ☑ Step 1 — bd-09a · Schema + AutoMap foundation

- **Beads:** `dbsync-ybe`
- **GitHub:** [#9](https://github.com/kentoespdam/dbsync/issues/9)
- **Branch suggest:** `feat/bd-09a-automap-foundation`
- **Plan section:** `bd-09a — Schema + AutoMap foundation`
- **Touches:** `internal/mysql/schema.go`, `internal/storage/mappings.go`, dan test mereka. **TIDAK** ada perubahan di `internal/tui/**`.

### Pre-work
- [x] Buka & baca [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md) — minimal section "Synthetic rows" + "bd-09a".
- [x] Tidak perlu `context7` untuk issue ini (pure Go stdlib + driver). Sebutkan di PR description.

### Implementasi
- [x] `Column.ColumnType string` (raw `COLUMN_TYPE` dari `INFORMATION_SCHEMA.COLUMNS`).
- [x] `DescribeColumns` ambil `COLUMN_TYPE`.
- [x] `Column.IsBool() bool` → `strings.EqualFold(ColumnType, "tinyint(1)")`.
- [x] `Column.EnumValues() []string` → parse `enum('a','b','c')` (regex / strings.Split).
- [x] `AutoMap` rewrite: SELALU append entry untuk setiap dest col; jika tidak ada match → synthetic `Mapping{SourceColumn: invalid, DefaultValue: invalid}`.

### Test
- [x] `internal/mysql/schema_test.go` — `IsBool` & `EnumValues` untuk berbagai input.
- [x] `internal/storage/mappings_test.go` — `AutoMap` synthetic row coverage.
- [x] `go test ./internal/mysql/... ./internal/storage/...` HIJAU.

### Close-out
- [x] PR merged ke `main`.
- [x] `bd close dbsync-ybe`.
- [x] `git push && bd dolt push`.

---

## ☑ Step 2 — bd-09b · Mapping editor table redesign

- **Beads:** `dbsync-zcw`
- **GitHub:** [#10](https://github.com/kentoespdam/dbsync/issues/10)
- **Branch suggest:** `feat/bd-09b-table-redesign`
- **Plan section:** `bd-09b — Table redesign`
- **Prerequisite:** Step 1 merged ✅. Jangan mulai sebelum itu.

### Pre-work
- [x] Baca section "Layout", "Status ikon (4 state)", "Filter & navigasi", "Discard guard" di plan.
- [x] `context7` query: `github.com/charmbracelet/bubbles/table` (custom row styling), `github.com/charmbracelet/lipgloss` (foreground per cell, border + padding). Catat hasil di PR description.

### Implementasi
- [x] Kolom `ST` (width 4) di table; render via lipgloss color.
- [x] `mappingStatus(mp, dc) (icon string, color lipgloss.Color)` — 4 state sesuai tabel di plan.
- [x] Header 3-line: title, `src.X → dst.Y`, stats `N cols • M mapped • K default • W ⚠ unresolved`.
- [x] Help bar 2-line per grup logis.
- [x] Key `/` → filter mode (filter by `DestColumn`).
- [x] Key `w` → toggle warnings-only.
- [x] Key `N` → jump cursor ke ⚠ berikutnya (wrap).
- [x] `esc` saat `dirty=true` → `showDiscardConfirm` (reuse `app.go:124-140`).
- [x] `recomputeWarnings` jalan atas synthetic rows.

### QA manual
- [x] Tabel `users` dengan dest col `isDeleted` (TINYINT(1) NOT NULL) → row muncul dengan ikon ⚠.
- [x] Edit row biasa → stats di header update.
- [x] `/` filter, `esc` keluar filter (tidak keluar editor).
- [x] `w` toggle warnings-only.
- [x] `N` wrap to first ⚠.
- [x] `esc` dengan dirty → confirm modal.

### Close-out
- [x] PR merged.
- [x] `bd close dbsync-zcw`.
- [x] `git push && bd dolt push`.

---

## ☑ Step 3 — bd-09c · Modal edit + hard-block save

- **Beads:** `dbsync-lxq`
- **GitHub:** [#11](https://github.com/kentoespdam/dbsync/issues/11)
- **Branch suggest:** `feat/bd-09c-modal-edit`
- **Plan section:** `bd-09c — Modal edit (type-aware default)` + "Save validation (hard-block)"
- **Prerequisite:** Step 2 merged ✅.

### Pre-work
- [x] Baca section "Edit modal" + "Save validation (hard-block)" di plan.
- [x] `context7` query: `github.com/charmbracelet/bubbletea` (modal overlay & message routing), `github.com/charmbracelet/bubbles/list` (filtered + custom delegate), `github.com/charmbracelet/lipgloss` (`Place` overlay center). Catat di PR description.

### Implementasi
- [x] Rewrite `internal/tui/mapping_edit_form.go` jadi modal (`lipgloss.Place(... Center, Center, ...)`).
- [x] Source field: `list.Model` filter + `(none)` di top + auto-suggest (Levenshtein / prefix) terhadap nama dest col.
- [x] Default widget switch:
  - [x] `dc.IsBool()` → radio 3-state (true / false / (empty)).
  - [x] `len(dc.EnumValues()) > 0` → list pick dari nilai valid.
  - [x] Else → `textinput` + helper hint `NOW(), CURRENT_TIMESTAMP, integer, "string"`.
- [x] Inline validation di modal: NOT NULL & source `(none)` & default kosong → error inline (block save modal).
- [x] `mapping_editor.save()`: hard-block kalau ada `⚠` (NOT NULL no source no default). Format error: `Cannot save: N NOT NULL columns unresolved (col1, col2)`.
- [x] Tombol `n` (dead flag di existing) → sambungkan ke modal atau hapus (sesuaikan dengan synthetic-row approach).

### QA manual
- [x] Edit `isDeleted` → modal radio true/false/(empty).
- [x] Edit kolom ENUM → modal dropdown nilai valid.
- [x] Edit kolom string → textinput + helper hint.
- [x] Modal `esc` cancel, `enter` save.
- [x] Source picker auto-suggest jalan.
- [x] Save dengan ≥1 ⚠ → save di-block (error message visible; toast UI di Step 4).
- [x] Synthetic row tidak diisi → tidak ke DB.

### Close-out
- [x] PR merged.
- [x] `bd close dbsync-lxq`.
- [x] `git push && bd dolt push`.

---

## ☑ Step 4 — bd-09d · Save feedback toast

- **Beads:** `dbsync-vsk`
- **GitHub:** [#12](https://github.com/kentoespdam/dbsync/issues/12)
- **Branch suggest:** `feat/bd-09d-toast`
- **Plan section:** `bd-09d — Save feedback (toast)`
- **Prerequisite:** Step 3 merged ✅.

### Pre-work
- [x] Baca section "Save feedback (toast)" di plan.
- [x] `context7` query: `github.com/charmbracelet/bubbletea` (`tea.Tick` scheduled message), `github.com/charmbracelet/lipgloss` (border + bg inline). Catat di PR description.

### Implementasi
- [x] `successMsg` → `struct { message string }`.
- [x] `model` field: `toastMsg string`, `toastUntil time.Time`.
- [x] `View()` overlay toast di footer kalau `time.Now().Before(toastUntil)`.
- [x] Handler `successMsg`: set state + `tea.Tick(3*time.Second, ...)` → `clearToastMsg{}`.
- [x] Handler `clearToastMsg`: reset state.
- [x] File baru `internal/tui/toast.go` (helper render + tipe message).
- [x] `mapping_editor.save()` kirim `successMsg{message: "✓ Saved N mappings"}`. Error case (hard-block dari bd-09c) → error toast merah.

### QA manual
- [x] Save sukses → toast hijau `✓ Saved N mappings` 3 detik.
- [x] Toast tetap visible setelah pop history.
- [x] Toast tidak block input.
- [x] Save di-block → toast merah dengan list kolom unresolved.

### Close-out
- [x] PR merged.
- [x] `bd close dbsync-vsk`.
- [x] `git push && bd dolt push`.

---

## ☑ Step 5 — bd-09e · Table picker UX (Flow A)

- **Beads:** `dbsync-0dt`
- **GitHub:** [#13](https://github.com/kentoespdam/dbsync/issues/13)
- **Branch suggest:** `feat/bd-09e-table-picker-ux`
- **Plan section:** `bd-09e — Table picker UX (Flow A: Tables & Mappings)`
- **Prerequisite:** Step 4 merged ✅.
- **Touches:** `internal/tui/table_picker.go` (extend). **TIDAK** ada perubahan di `mapping_editor*.go` / `mapping_edit_form*.go`.

### Pre-work
- [x] Baca section "bd-09e — Table picker UX" + ASCII layout di plan.
- [x] `context7` query: `github.com/charmbracelet/bubbles/list` (custom delegate + dynamic SetItems), `github.com/charmbracelet/bubbles/textinput` (border style on focus/blur), `github.com/charmbracelet/lipgloss` (JoinVertical untuk segmen header/filter/list/help). Catat di PR description.

### Implementasi
- [x] Extend `tablePickerModel`: `filterInput textinput.Model`, `unmappedOnly bool`, `focused int`, `allItems []tableItem` (cache full set).
- [x] Custom delegate `tableItemDelegate` (kolom ikon ✓/○ + render badge `[no-pk]` kuning).
- [x] `View()` 4-segmen: stats (2-line), filter input (bordered), list, help bar (1-line).
- [x] `Update()`: tab toggle focus; route msg by `focused`; tangani `u`/`r`/`enter` saat list focused.
- [x] `applyFilter()`: substring AND unmapped filter; live update stats counts.
- [x] Stats line 1 hitung atas `allItems` (absolut), line 2 hanya muncul kalau filter aktif.
- [x] Reload (`r`) pertahankan filter state (text + toggle).
- [x] **Tidak break Flow B** (`Select Table to Sync`): semua perubahan jalan untuk flow B juga; item `--- SYNC ALL MAPPED TABLES ---` tetap di top, ikut filter sebagai `✓` (mapped).

### QA manual
- [x] Masuk `Tables & Mappings` → filter input langsung focused, border pink.
- [x] Ketik `user` → list menyempit; stats line 2 muncul `Filter: 'user' • showing N`.
- [x] Tab → focus pindah list, border filter abu, cursor highlight di list.
- [x] Di list focus: `u` → toggle unmapped-only; stats line 2 update.
- [x] Filter teks + `u` aktif bersamaan → AND compose; stats `Filter: 'user' • unmapped-only • showing N`.
- [x] Tabel mapped tampil `✓` hijau; unmapped `○` abu; tabel tanpa PK tetap punya `[no-pk]` kuning di description.
- [x] `r` reload re-query + recompute; filter teks + toggle TIDAK reset.
- [x] Stats line 1 absolut (`40 tables • 12 mapped • 28 unmapped • 2 no-pk`) tidak ikut filter.
- [x] Selesai edit mapping → balik ke screen → `r` → tabel target sekarang ikon `✓`.
- [x] Flow B (`Run Sync → Select Table to Sync`) tidak regresi.

### Close-out
- [x] PR merged.
- [x] `bd close dbsync-0dt`.
- [x] `git push && bd dolt push`.

---

## Final QA (post bd-09e, dijalankan oleh reviewer terakhir)

Checklist lengkap ada di [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md) section "Manual QA". Ringkas:

- [x] `isDeleted` (TINYINT(1) NOT NULL) muncul sebagai row ⚠.
- [x] Edit `isDeleted` set `false` → save → toast hijau → reload → ikon jadi ●.
- [x] Save dengan ⚠ masih ada → toast merah, mapping tidak ter-save.
- [x] `/`, `w`, `N` semua jalan.
- [x] ENUM column → dropdown nilai valid.
- [x] Esc dengan dirty → confirm "Discard?".
- [x] Resize terminal → layout tetap usable.
- [ ] Table picker (Flow A): filter input always-on, ikon ✓/○, stats 2-line, toggle `u`, tab switch focus, badge `[no-pk]`.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan.
- **Test gagal di area yang bukan scope issue** → flag di PR description, jangan tutup-tutupi.
