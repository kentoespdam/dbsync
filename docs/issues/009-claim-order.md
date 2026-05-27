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
bd-09a (foundation)  ──►  bd-09b (table redesign)  ──►  bd-09c (modal + validation)  ──►  bd-09d (toast)
   #9                       #10                          #11                              #12
```

---

## ☐ Step 1 — bd-09a · Schema + AutoMap foundation

- **Beads:** `dbsync-ybe`
- **GitHub:** [#9](https://github.com/kentoespdam/dbsync/issues/9)
- **Branch suggest:** `feat/bd-09a-automap-foundation`
- **Plan section:** `bd-09a — Schema + AutoMap foundation`
- **Touches:** `internal/mysql/schema.go`, `internal/storage/mappings.go`, dan test mereka. **TIDAK** ada perubahan di `internal/tui/**`.

### Pre-work
- [ ] Buka & baca [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md) — minimal section "Synthetic rows" + "bd-09a".
- [ ] Tidak perlu `context7` untuk issue ini (pure Go stdlib + driver). Sebutkan di PR description.

### Implementasi
- [ ] `Column.ColumnType string` (raw `COLUMN_TYPE` dari `INFORMATION_SCHEMA.COLUMNS`).
- [ ] `DescribeColumns` ambil `COLUMN_TYPE`.
- [ ] `Column.IsBool() bool` → `strings.EqualFold(ColumnType, "tinyint(1)")`.
- [ ] `Column.EnumValues() []string` → parse `enum('a','b','c')` (regex / strings.Split).
- [ ] `AutoMap` rewrite: SELALU append entry untuk setiap dest col; jika tidak ada match → synthetic `Mapping{SourceColumn: invalid, DefaultValue: invalid}`.

### Test
- [ ] `internal/mysql/schema_test.go` — `IsBool` & `EnumValues` untuk berbagai input.
- [ ] `internal/storage/mappings_test.go` — `AutoMap` synthetic row coverage.
- [ ] `go test ./internal/mysql/... ./internal/storage/...` HIJAU.

### Close-out
- [ ] PR merged ke `main`.
- [ ] `bd close dbsync-ybe`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 2 — bd-09b · Mapping editor table redesign

- **Beads:** `dbsync-zcw`
- **GitHub:** [#10](https://github.com/kentoespdam/dbsync/issues/10)
- **Branch suggest:** `feat/bd-09b-table-redesign`
- **Plan section:** `bd-09b — Table redesign`
- **Prerequisite:** Step 1 merged ✅. Jangan mulai sebelum itu.

### Pre-work
- [ ] Baca section "Layout", "Status ikon (4 state)", "Filter & navigasi", "Discard guard" di plan.
- [ ] `context7` query: `github.com/charmbracelet/bubbles/table` (custom row styling), `github.com/charmbracelet/lipgloss` (foreground per cell, border + padding). Catat hasil di PR description.

### Implementasi
- [ ] Kolom `ST` (width 4) di table; render via lipgloss color.
- [ ] `mappingStatus(mp, dc) (icon string, color lipgloss.Color)` — 4 state sesuai tabel di plan.
- [ ] Header 3-line: title, `src.X → dst.Y`, stats `N cols • M mapped • K default • W ⚠ unresolved`.
- [ ] Help bar 2-line per grup logis.
- [ ] Key `/` → filter mode (filter by `DestColumn`).
- [ ] Key `w` → toggle warnings-only.
- [ ] Key `N` → jump cursor ke ⚠ berikutnya (wrap).
- [ ] `esc` saat `dirty=true` → `showDiscardConfirm` (reuse `app.go:124-140`).
- [ ] `recomputeWarnings` jalan atas synthetic rows.

### QA manual
- [ ] Tabel `users` dengan dest col `isDeleted` (TINYINT(1) NOT NULL) → row muncul dengan ikon ⚠.
- [ ] Edit row biasa → stats di header update.
- [ ] `/` filter, `esc` keluar filter (tidak keluar editor).
- [ ] `w` toggle warnings-only.
- [ ] `N` wrap to first ⚠.
- [ ] `esc` dengan dirty → confirm modal.

### Close-out
- [ ] PR merged.
- [ ] `bd close dbsync-zcw`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 3 — bd-09c · Modal edit + hard-block save

- **Beads:** `dbsync-lxq`
- **GitHub:** [#11](https://github.com/kentoespdam/dbsync/issues/11)
- **Branch suggest:** `feat/bd-09c-modal-edit`
- **Plan section:** `bd-09c — Modal edit (type-aware default)` + "Save validation (hard-block)"
- **Prerequisite:** Step 2 merged ✅.

### Pre-work
- [ ] Baca section "Edit modal" + "Save validation (hard-block)" di plan.
- [ ] `context7` query: `github.com/charmbracelet/bubbletea` (modal overlay & message routing), `github.com/charmbracelet/bubbles/list` (filtered + custom delegate), `github.com/charmbracelet/lipgloss` (`Place` overlay center). Catat di PR description.

### Implementasi
- [ ] Rewrite `internal/tui/mapping_edit_form.go` jadi modal (`lipgloss.Place(... Center, Center, ...)`).
- [ ] Source field: `list.Model` filter + `(none)` di top + auto-suggest (Levenshtein / prefix) terhadap nama dest col.
- [ ] Default widget switch:
  - [ ] `dc.IsBool()` → radio 3-state (true / false / (empty)).
  - [ ] `len(dc.EnumValues()) > 0` → list pick dari nilai valid.
  - [ ] Else → `textinput` + helper hint `NOW(), CURRENT_TIMESTAMP, integer, "string"`.
- [ ] Inline validation di modal: NOT NULL & source `(none)` & default kosong → error inline (block save modal).
- [ ] `mapping_editor.save()`: hard-block kalau ada `⚠` (NOT NULL no source no default). Format error: `Cannot save: N NOT NULL columns unresolved (col1, col2)`.
- [ ] Tombol `n` (dead flag di existing) → sambungkan ke modal atau hapus (sesuaikan dengan synthetic-row approach).

### QA manual
- [ ] Edit `isDeleted` → modal radio true/false/(empty).
- [ ] Edit kolom ENUM → modal dropdown nilai valid.
- [ ] Edit kolom string → textinput + helper hint.
- [ ] Modal `esc` cancel, `enter` save.
- [ ] Source picker auto-suggest jalan.
- [ ] Save dengan ≥1 ⚠ → save di-block (error message visible; toast UI di Step 4).
- [ ] Synthetic row tidak diisi → tidak ke DB.

### Close-out
- [ ] PR merged.
- [ ] `bd close dbsync-lxq`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 4 — bd-09d · Save feedback toast

- **Beads:** `dbsync-vsk`
- **GitHub:** [#12](https://github.com/kentoespdam/dbsync/issues/12)
- **Branch suggest:** `feat/bd-09d-toast`
- **Plan section:** `bd-09d — Save feedback (toast)`
- **Prerequisite:** Step 3 merged ✅.

### Pre-work
- [ ] Baca section "Save feedback (toast)" di plan.
- [ ] `context7` query: `github.com/charmbracelet/bubbletea` (`tea.Tick` scheduled message), `github.com/charmbracelet/lipgloss` (border + bg inline). Catat di PR description.

### Implementasi
- [ ] `successMsg` → `struct { message string }`.
- [ ] `model` field: `toastMsg string`, `toastUntil time.Time`.
- [ ] `View()` overlay toast di footer kalau `time.Now().Before(toastUntil)`.
- [ ] Handler `successMsg`: set state + `tea.Tick(3*time.Second, ...)` → `clearToastMsg{}`.
- [ ] Handler `clearToastMsg`: reset state.
- [ ] File baru `internal/tui/toast.go` (helper render + tipe message).
- [ ] `mapping_editor.save()` kirim `successMsg{message: "✓ Saved N mappings"}`. Error case (hard-block dari bd-09c) → error toast merah.

### QA manual
- [ ] Save sukses → toast hijau `✓ Saved N mappings` 3 detik.
- [ ] Toast tetap visible setelah pop history.
- [ ] Toast tidak block input.
- [ ] Save di-block → toast merah dengan list kolom unresolved.

### Close-out
- [ ] PR merged.
- [ ] `bd close dbsync-vsk`.
- [ ] `git push && bd dolt push`.

---

## Final QA (post bd-09d, dijalankan oleh reviewer terakhir)

Checklist lengkap ada di [`009-mapping-editor-ux.md`](./009-mapping-editor-ux.md) section "Manual QA". Ringkas:

- [ ] `isDeleted` (TINYINT(1) NOT NULL) muncul sebagai row ⚠.
- [ ] Edit `isDeleted` set `false` → save → toast hijau → reload → ikon jadi ●.
- [ ] Save dengan ⚠ masih ada → toast merah, mapping tidak ter-save.
- [ ] `/`, `w`, `N` semua jalan.
- [ ] ENUM column → dropdown nilai valid.
- [ ] Esc dengan dirty → confirm "Discard?".
- [ ] Resize terminal → layout tetap usable.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan.
- **Test gagal di area yang bukan scope issue** → flag di PR description, jangan tutup-tutupi.
