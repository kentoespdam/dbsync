# Issue 014 — TUI Value Map: form fixes + list-level enum mismatch warning

**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md) · [`CONTEXT.md`](../../CONTEXT.md)
**ADR yang relevan:** [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md)
**Plan sebelumnya:** [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md) (bd-13e melahirkan TUI form; issue ini memperbaiki + extend)
**Claim order:** [`014-claim-order.md`](./014-claim-order.md)

---

## ⚠️ Wajib dibaca dulu (sebelum koding)

1. [`CONTEXT.md`](../../CONTEXT.md) — orientasi. Section **Glossary → "Value Map"** = definisi kanonik.
2. [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md) — semantik value_map (strict miss = error, validasi dest).
3. [`CLAUDE.md`](../../CLAUDE.md) — DRY comments, max 120 baris per file/function, test wajib `storage`, manual QA `tui`, session close MANDATORY push.
4. [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md) — terutama bd-13d (`EnumDomainMismatch`) dan bd-13e (struktur form).

**Skill/MCP wajib SEBELUM koding:**

- `context7` → query lib eksternal yang di-touch sub-issue (`github.com/charmbracelet/bubbles/textinput`, `github.com/charmbracelet/bubbles/list`, `github.com/charmbracelet/lipgloss`). Catat hasil di PR description.
- `gitnexus` → `gitnexus_impact({target, direction:"upstream"})` SEBELUM edit symbol; `gitnexus_detect_changes()` SEBELUM commit. **HIGH/CRITICAL risk → STOP & lapor user.**
- Eksplorasi cepat: `gitnexus_query` / `gitnexus_context`. **Jangan** `ls -R` / grep buta.

---

## Aturan main (NON-NEGOTIABLE)

- **Satu agent = satu sub-issue.** Jangan paralel.
- **Jangan keluar plan.** Touches per sub-issue eksplisit. Kebutuhan tambahan → file issue baru, **jangan** improvisasi.
- **Jangan edit file di luar `Touches`.** Termasuk test, dokumen, CI.
- **Backward-compat.** Mapping existing tanpa enum mismatch / tanpa value_map MUST tetap berjalan persis. Existing tests harus hijau tanpa modifikasi.
- **Semantik value_map locked oleh ADR 0005.** Jangan ubah lookup logic engine. Sub-issue ini murni TUI + helper ekspor.
- **DRY comments.** Reference `bd-14X` di kode, jangan duplikasi deskripsi.
- **Patuh dependency chain** — bd-14b BLOCKED sampai bd-14a merged.
- **Session close MANDATORY**: `git pull --rebase && bd dolt push && git push && git status` harus return clean & up-to-date.

---

## Konteks masalah

### Cacat 1 — Value Map form UX rusak (bd-14a)

User report: "edit kolom `status` type ENUM dengan source/dest beda value. Tab sampai Value Map, ada `> Dest value...` muncul di dalam border. Tekan banyak tombol tidak ada respons."

**Root cause analysis (file: `internal/tui/mapping_edit_form_update.go`):**

```go
case "tab", "shift+tab":
    maxFocus := 1
    if m.hasValueMap { maxFocus = 2 }
    m.focused = (m.focused + 1) % (maxFocus + 1)
    if m.focused == 1 && !m.isBool && !m.isEnum { m.input.Focus() } else { m.input.Blur() }
    if m.focused == 2 { m.valueMapEditing = 1 }   // ← bug 2: auto-add mode
    if m.focused != 2 { m.valueMapEditing = 0 }
    return m, nil
```

1. **Bug fokus**: `m.valueMapInput.Focus()` **tidak pernah** dipanggil di handler tab. Hanya dipanggil di `case "a"` (baris 67-72). Bubbletea `textinput` butuh `.Focus()` agar terima KeyMsg.
2. **Bug mode**: tab langsung set `valueMapEditing = 1`. User tidak bisa browse pair existing dulu (mode editing=0). Add/remove keybind (`a` / `x`) cuma aktif di editing=0.
3. **Bug label**: di `mapping_edit_form.go:138` placeholder = `"Dest value..."`. Tapi saat editing=1, handler enter (update.go:27-34) menyimpan `m.valueMapInput.Value()` sebagai **`Source`** pair. Label menyesatkan.

### Cacat 2 — List view tidak warn enum mismatch (bd-14b)

`internal/tui/mapping_editor_view.go` → `mappingStatus()` (baris 92-103) hanya punya 4 kategori: `✓` (mapped), `●` (default-only), `⚠` (broken NOT NULL), `○` (skipped nullable). Tidak ada cabang untuk "source ENUM dengan domain beda dari dest ENUM dan value_map belum cover semua".

Akibat: user baru tahu masalah saat run sync. Struct `EnumDomainMismatch` + helper `stringSetsEqual` sudah ada di `internal/storage/mappings.go` (bd-13d), reuse di TUI.

---

## Solusi (locked design — JANGAN improvisasi)

### bd-14a: form fixes

1. **Hapus auto-set `valueMapEditing = 1`** di tab handler. Saat tab masuk focus=2, biarkan `valueMapEditing = 0` (browse mode).
2. **`a`** (existing) → masuk add mode (editing=1), `Focus()` valueMapInput.
3. **`e`** (BARU) → kalau editing=0 dan ada pair selected, prefill input dengan `pair.Source`, set `valueMapEditing = 1`, set `m.valueMapEditIdx = m.valueMapCursor`, `Focus()`. Saat enter (mode dest-picker → mode 2), kalau `valueMapEditIdx >= 0` → **replace** pair existing, bukan append. Reset `valueMapEditIdx = -1`.
4. **`↑↓`** (BARU saat editing=0) → geser `valueMapCursor` di range `[0, len(valueMapPairs)-1]`. Saat editing=2 perilaku existing tetap.
5. **Placeholder mode-aware**: saat masuk editing=1, set `m.valueMapInput.Placeholder = "Source value..."` (langsung di handler `a`/`e`).
6. **Esc dari editing=1** → set `valueMapEditIdx = -1`, clear input value, kembali ke editing=0 (sudah ada, tapi pastikan idx ter-reset).
7. **View hint**: kalau editing=0 tampilkan `a: add  e: edit  x: remove  ↑↓: browse` (gantikan baris 73).

**Tidak diubah:**
- Semantik mode 2 (dest picker dengan ↑↓ + enter) tetap.
- `handleApply()` JSON serialization tetap.
- `storage.ValidateMapping()` tetap dipanggil saat save.
- Tab cycle: 0→1→2→0 (kalau hasValueMap), atau 0→1→0.

### bd-14b: list warning + save guard

1. **Ekspor helper.** Pilih satu (PIC putuskan saat koding berdasarkan minimal blast radius, default = opsi A):
   - **Opsi A** (default): `internal/storage/mappings.go` → rename `stringSetsEqual` → `StringSetsEqual`. Update internal call site. Tidak ada signature lain berubah.
   - **Opsi B** (kalau opsi A breaks ≥3 caller): tambah method `(c mysql.Column) EnumDomainEquals(other Column) bool` di `internal/mysql/schema.go`. Storage tetap pakai helper lama. TUI pakai method.
2. **Helper baru di TUI (`mapping_editor_view.go`):**
   ```go
   func (m mappingEditorModel) enumMismatch(mp storage.Mapping, dc mysql.Column) bool { ... }
   ```
   Return `true` kalau: source col valid AND source.EnumValues() non-empty AND dest.EnumValues() non-empty AND set tidak sama AND value_map tidak cover semua source values.
3. **Coverage check helper:**
   ```go
   func valueMapCoversSource(valueMap sql.NullString, srcEnum []string) bool { ... }
   ```
   - `!valueMap.Valid` → false.
   - JSON unmarshal gagal → false.
   - Untuk setiap v ∈ srcEnum, kalau `_, ok := vmap[v]; !ok` → false.
   - Semua tercover → true.
4. **Extend `mappingStatus`:** tambah cabang **sebelum** return `✓`:
   ```go
   if mp.SourceColumn.Valid {
       if m.enumMismatch(mp, dc) {
           return "⚡", lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Yellow
       }
       return "✓", ...
   }
   ```
5. **Extend `statusText`:** tambah `case "⚡": return "enum domain mismatch — value_map incomplete"`.
6. **Extend header counter** di `renderHeader`: tambah variable `mismatch` di switch loop, tampilkan di stats line: `"%d cols • %d mapped • %d default • %d ⚡ mismatch • %d ⚠ unresolved"`.
7. **Save guard** di `mapping_editor_update.go` `save()`: hitung jumlah mismatch (loop `m.mappings`, panggil `m.enumMismatch`). Kalau > 0 → set `m.err` atau `m.statusMsg` ke `"cannot save: N enum mismatches need value_map"` dan return tanpa write. **Tidak** mengganggu existing unresolved-block kalau ada.

**Tidak diubah:**
- Engine `Resolve` semantik (ADR 0005 lock).
- `storage.ValidateMapping` body.
- `AutoMap` flow.
- CLI `mapping auto`.

---

## Pecah sub-issue

```
bd-14a (form fixes) ──► bd-14b (list warning + save guard)
```

Chain linear: bd-14b **HARUS** menunggu bd-14a merged.

---

## bd-14a — TUI Form: fix 3 cacat (focus, browse, label)

**Beads:** `dbsync-e44` · **GitHub:** [#38](https://github.com/kentoespdam/dbsync/issues/38)
**Branch suggest:** `feat/bd-14a-value-map-form-fixes`

**Touches:**
- `internal/tui/mapping_edit_form.go` (tambah field `valueMapEditIdx int`)
- `internal/tui/mapping_edit_form_update.go` (tab handler, `a`/`e`/`x`/`↑↓` handlers, enter handler untuk replace path)
- `internal/tui/mapping_edit_form_view.go` (hint text + placeholder akan diset dari update.go bukan di sini)

**TIDAK menyentuh:**
- `internal/tui/mapping_editor.go`, `mapping_editor_view.go`, `mapping_editor_update.go`
- `internal/storage/**`, `internal/engine/**`, `internal/cli/**`, `internal/mysql/**`

### Pre-work
- [ ] `gitnexus_context({name: "mappingEditFormModel"})` + `gitnexus_impact({target: "mappingEditFormModel"})`. HIGH/CRITICAL → STOP & lapor.
- [ ] `gitnexus_context({name: "Update"})` di scope tui untuk konfirmasi tidak ada caller lain.
- [ ] `context7` query `github.com/charmbracelet/bubbles/textinput` topik "Focus, Blur, Placeholder update, KeyMsg routing". Catat di PR.
- [ ] `context7` query `github.com/charmbracelet/bubbles/list` topik "filter state vs key dispatch". Catat di PR.

### Implementasi (acceptance criteria)
- [ ] Tab masuk focus=2 → border highlight, `valueMapEditing = 0`, **valueMapInput TIDAK ter-focus** (browse mode).
- [ ] Tekan `a` di focus=2/editing=0 → editing=1, `valueMapInput.Focus()` dipanggil, placeholder = `"Source value..."`, input kosong.
- [ ] Tekan `e` di focus=2/editing=0 dengan ≥1 pair → editing=1, `valueMapEditIdx = cursor`, input prefill `pair.Source`, placeholder = `"Source value..."`, `Focus()`.
- [ ] Tekan `x` di focus=2/editing=0 → existing perilaku tetap (delete pair under cursor).
- [ ] Tekan `↑`/`↓` di focus=2/editing=0 dengan ≥1 pair → cursor pindah dalam range. (Existing handler editing=2 tetap.)
- [ ] Enter di editing=1 → switch ke editing=2 (existing).
- [ ] Enter di editing=2 dengan `valueMapEditIdx >= 0` → **replace** `valueMapPairs[idx]`. Set `valueMapEditIdx = -1`. Clear input.
- [ ] Enter di editing=2 dengan `valueMapEditIdx == -1` → **append** (existing).
- [ ] Esc di editing>0 → clear input, reset `valueMapEditIdx = -1`, editing=0.
- [ ] View hint saat focus=2/editing=0: `a: add  e: edit  x: remove  ↑↓: browse`.
- [ ] Tab keluar focus=2 → `valueMapInput.Blur()` jika sedang focus. `valueMapEditing = 0`.

### Test (manual QA — bukti screenshot/log di PR)
- [ ] Open ENUM dest col tanpa value_map → `passthrough (no mapping)`, tab masuk Value Map, `a` → bisa ketik.
- [ ] Add pair `Draft=DRAFT` via `a`+enter+enter → muncul di list.
- [ ] Add 2nd pair `Ditampilkan=PUBLISHED` → list punya 2.
- [ ] Tab keluar lalu balik → cursor di pair pertama, `↓` pindah pair kedua.
- [ ] `e` di pair kedua → input prefill `Ditampilkan`, ubah jadi `Ditampilkan2`, enter, pilih `DELETED`, enter → pair kedua jadi `Ditampilkan2=DELETED` (replace, bukan append).
- [ ] `x` di pair pertama → tersisa 1 pair.
- [ ] Esc dari mid-add → tidak ada pair tambahan.
- [ ] Kolom non-ENUM → section TIDAK muncul (existing).
- [ ] Save → toast hijau, reload form → pair persist.

### Close-out
- [ ] `gitnexus_detect_changes()` → attach di PR (hanya symbol scope).
- [ ] PR merged ke `main`.
- [ ] `bd close dbsync-e44`.
- [ ] `git push && bd dolt push && git status` clean.

---

## bd-14b — TUI List: enum domain mismatch warning + save guard

**Beads:** `dbsync-tqj` · **GitHub:** [#39](https://github.com/kentoespdam/dbsync/issues/39)
**Branch suggest:** `feat/bd-14b-list-enum-mismatch-warning`
**Prerequisite:** bd-14a merged ✅.

**Touches:**
- `internal/storage/mappings.go` (opsi A: rename `stringSetsEqual` → `StringSetsEqual`; update internal callers) ATAU `internal/mysql/schema.go` (opsi B: tambah `EnumDomainEquals` method)
- `internal/tui/mapping_editor_view.go` (helper `enumMismatch`, `valueMapCoversSource`, extend `mappingStatus`, `statusText`, `renderHeader` counter)
- `internal/tui/mapping_editor_update.go` (save guard)
- Test: `internal/storage/mappings_test.go` (kalau opsi A: pastikan rename tidak break test) ATAU `internal/mysql/schema_test.go` (kalau opsi B: test `EnumDomainEquals`)

**TIDAK menyentuh:**
- `internal/engine/**`, `internal/cli/**`
- `internal/tui/mapping_edit_form*.go` (sudah scope bd-14a)
- ADR file (`docs/adr/0005*.md`)

### Pre-work
- [ ] `gitnexus_context({name: "stringSetsEqual"})` + `gitnexus_impact({target: "stringSetsEqual"})`. Tentukan opsi A vs B berdasarkan blast radius. HIGH/CRITICAL → STOP & lapor.
- [ ] `gitnexus_context({name: "mappingStatus"})` + `gitnexus_impact({target: "mappingStatus"})`.
- [ ] `gitnexus_context({name: "renderHeader"})` (di mapping_editor_view).
- [ ] `gitnexus_context({name: "save"})` di mapping_editor_update untuk pola error/statusMsg existing.
- [ ] `context7` query `github.com/charmbracelet/lipgloss` topik "Style.Foreground color codes". Catat di PR.

### Implementasi (acceptance criteria)
- [ ] Helper exported (opsi A `StringSetsEqual` atau opsi B `Column.EnumDomainEquals`). Test unit untuk opsi B kalau dipilih.
- [ ] `valueMapCoversSource(valueMap sql.NullString, srcEnum []string) bool` di `mapping_editor_view.go`. Behavior: invalid/parse-fail/missing key → false; all covered → true.
- [ ] `mappingEditorModel.enumMismatch(mp, dc) bool`: true iff source col valid AND `srcCol.EnumValues()` non-empty AND `dc.EnumValues()` non-empty AND set tidak sama AND `!valueMapCoversSource(mp.ValueMap, srcCol.EnumValues())`.
- [ ] `mappingStatus`: kalau `mp.SourceColumn.Valid` cek `enumMismatch` dulu → `("⚡", yellow lipgloss Color("214"))`. Else fallback `✓` existing.
- [ ] `statusText`: tambah `case "⚡": return "enum domain mismatch — value_map incomplete"`.
- [ ] `renderHeader` stats line: tambah counter mismatch. Format: `"%d cols • %d mapped • %d default • %d ⚡ mismatch • %d ⚠ unresolved"`.
- [ ] `save()` di mapping_editor_update.go: count mismatch; jika > 0 → tolak save dengan pesan `"cannot save: N enum mismatches need value_map"` (gunakan pola error/statusMsg yang sudah ada di file itu, JANGAN bikin field baru kalau tidak perlu). Existing block "unresolved" tetap.

### Test wajib
- [ ] `storage`: kalau opsi A — pastikan rename `StringSetsEqual` tidak break existing tests (run `go test ./internal/storage/...`).
- [ ] `mysql`: kalau opsi B — tambah test `EnumDomainEquals` (3 case: identik, beda case, non-ENUM column).
- [ ] `tui`: tidak wajib (per CLAUDE.md). Bukti via manual QA.

### Manual QA (bukti screenshot di PR)
- [ ] Tabel dengan source ENUM `('Draft','Ditampilkan')` + dest ENUM `('DRAFT','PUBLISHED','DELETED')`, value_map kosong → status row = `⚡` kuning, statusText `"enum domain mismatch — value_map incomplete"`, counter `"1 ⚡ mismatch"`.
- [ ] Edit row, set value_map cover semua source (`Draft=DRAFT,Ditampilkan=PUBLISHED`) → status row jadi `✓`, counter mismatch turun jadi 0.
- [ ] Edit row, set value_map partial (`Draft=DRAFT` only) → status row tetap `⚡`.
- [ ] Domain identik (source ENUM `('A','B')`, dest ENUM `('A','B')`) → status `✓` tanpa value_map.
- [ ] Non-ENUM column (VARCHAR) → status existing tidak berubah.
- [ ] Coba `s` save saat ada `⚡` → tolak dengan pesan jelas.
- [ ] Save sukses setelah semua ⚡ resolved.

### Close-out
- [ ] `gitnexus_detect_changes()` → attach di PR (hanya symbol scope).
- [ ] PR merged ke `main`.
- [ ] `bd close dbsync-tqj`.
- [ ] `git push && bd dolt push && git status` clean.

---

## Final QA (post bd-14b, reviewer terakhir)

- [ ] End-to-end: tabel dengan enum mismatch terdeteksi di list (⚡), user edit form bisa tambah/edit/hapus pair tanpa hang, save guard menolak save saat masih mismatch, save lolos setelah resolve, sync run sukses.
- [ ] Backward-compat: tabel tanpa enum mismatch & tanpa value_map tetap berjalan persis seperti sebelum bd-14.
- [ ] `go test ./...` hijau.
- [ ] `go build -o dbsync ./cmd/dbsync` sukses.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan + ADR 0005.
- **Test gagal di area yang bukan scope** → flag di PR description, jangan tutup-tutupi.
- **`gitnexus_impact` HIGH/CRITICAL** → stop, lapor user, tunggu konfirmasi.
- **bd-14b ragu opsi A vs B** → default opsi A. Pindah ke B hanya kalau opsi A bikin > 3 caller berubah (catat di PR).
