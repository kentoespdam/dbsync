# Issue 014 — Claim order & progress checklist

**Parent plan:** [`014-tui-value-map-fixes.md`](./014-tui-value-map-fixes.md)
**ADR yang relevan:** [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md)
**Tujuan:** Pastikan 2 sub-issue dikerjakan **berurutan**, satu per satu, tanpa overlap. Agent claim ID berikutnya HANYA setelah PR sebelumnya merged ke `main`.

---

## ⚠️ Aturan main (baca dulu — sebelum apa-apa)

1. **Wajib baca dulu (urutan):**
   - [`CONTEXT.md`](../../CONTEXT.md) — orientasi + Glossary "Value Map".
   - [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md) — keputusan terkunci.
   - [`014-tui-value-map-fixes.md`](./014-tui-value-map-fixes.md) — plan detail sub-issue.
   - [`CLAUDE.md`](../../CLAUDE.md) — DRY comments, max 120 baris, test wajib, session close mandatory push.
2. **Skill/MCP wajib (per sub-issue):**
   - `gitnexus_impact({target, direction:"upstream"})` SEBELUM edit symbol. HIGH/CRITICAL → STOP & lapor.
   - `gitnexus_detect_changes()` SEBELUM commit. Pastikan hanya symbol scope.
   - `context7` query SEBELUM koding pakai lib eksternal yang relevan. Catat di PR description.
   - **Jangan** `ls -R` / grep buta — pakai `gitnexus_query` / `gitnexus_context`.
3. **Plan doc adalah sumber kebenaran.** Improvisasi di luar yang sudah locked = ditolak di review.
4. **Patuh dependency chain** — bd-14b BLOCKED sampai bd-14a closed & PR merged.
5. **Satu agent = satu sub-issue.** Jangan paralel di branch yang sama.
6. **Jangan edit file di luar `Touches` sub-issue.** Termasuk test/dokumen/CI. Kalau perlu — file issue baru.
7. **Patuh `CLAUDE.md` rules:** DRY comments (ref `bd-14X`, jangan duplikasi deskripsi), max 120 baris per file/function, no DI framework, no CGo.
8. **Backward-compat WAJIB.** Mapping tanpa enum mismatch / tanpa value_map MUST tetap berjalan persis. Test existing tidak boleh dimodifikasi.
9. **Workflow per issue:**
   ```bash
   bd update <bd-id> --claim
   git checkout -b feat/<bd-id>-<slug>
   # … kerja, commit kecil-kecil …
   go test ./...
   go build -o dbsync ./cmd/dbsync
   git push -u origin feat/<bd-id>-<slug>
   gh pr create --base main --title "<bd-id>: …" \
     --body "Closes #<gh-num>. Plan: docs/issues/014-tui-value-map-fixes.md"
   # … review + merge …
   bd close <bd-id>
   ```
10. **Session close MANDATORY** (lihat `CLAUDE.md` "Session Completion"):
    ```bash
    git pull --rebase
    bd dolt push
    git push
    git status   # MUST: "up to date with origin"
    ```
    Work NOT complete sampai `git push` sukses.

---

## Claim order (linear chain — JANGAN paralel)

```
bd-14a (form fixes)  ──►  bd-14b (list warning + save guard)
 dbsync-e44               dbsync-tqj
 #38                      #39
```

---

## ☐ Step 1 — bd-14a · TUI Form: fix 3 cacat (focus, browse, label)

- **Beads:** `dbsync-e44`
- **GitHub:** [#38](https://github.com/kentoespdam/dbsync/issues/38)
- **Branch suggest:** `feat/bd-14a-value-map-form-fixes`
- **Plan section:** `bd-14a — TUI Form` di [`014-tui-value-map-fixes.md`](./014-tui-value-map-fixes.md).
- **Touches:** `internal/tui/mapping_edit_form.go`, `internal/tui/mapping_edit_form_update.go`, `internal/tui/mapping_edit_form_view.go`.
- **TIDAK** menyentuh: `internal/tui/mapping_editor*.go`, `internal/storage/**`, `internal/engine/**`, `internal/cli/**`, `internal/mysql/**`.

### Pre-work
- [x] Baca `CONTEXT.md` + ADR 0005 + section `bd-14a` di plan.
- [x] `gitnexus_context({name: "mappingEditFormModel"})` + `gitnexus_impact({target: "mappingEditFormModel"})`. Catat di PR. HIGH/CRITICAL → STOP & lapor.
- [x] `context7` query `github.com/charmbracelet/bubbles/textinput` topik "Focus, Blur, Placeholder update, KeyMsg routing". Catat di PR.
- [x] `context7` query `github.com/charmbracelet/bubbles/list` topik "filter state vs key dispatch". Catat di PR.

### Implementasi
- [x] Tambah field `valueMapEditIdx int` (default -1 saat init di `newMappingEditFormModel`).
- [x] Tab handler: hapus baris `if m.focused == 2 { m.valueMapEditing = 1 }`. Tetap reset `valueMapEditing = 0` saat keluar focus 2. Kalau perlu, panggil `m.valueMapInput.Blur()` saat tab keluar focus 2 dengan editing > 0.
- [x] Handler `case "a"`: tetap, tambah `m.valueMapInput.Placeholder = "Source value..."` sebelum `Focus()`.
- [x] Handler baru `case "e"`: aktif kalau `focused == 2 && valueMapEditing == 0 && len(valueMapPairs) > 0 && valueMapCursor < len(valueMapPairs)`. Prefill `m.valueMapInput.SetValue(valueMapPairs[cursor].Source)`. Set `valueMapEditIdx = valueMapCursor`. Set placeholder, `Focus()`.
- [x] Handler `↑/↓`: kalau `focused == 2 && valueMapEditing == 0 && len(valueMapPairs) > 0`, geser `valueMapCursor` di range `[0, len(valueMapPairs)-1]`. (Existing handler editing == 2 tetap.)
- [x] Handler enter di editing=2: kalau `valueMapEditIdx >= 0` → replace `valueMapPairs[valueMapEditIdx]`, reset `valueMapEditIdx = -1`. Else append (existing).
- [x] Handler esc dari editing > 0: tambah `valueMapEditIdx = -1` di reset block existing.
- [x] View hint saat focused=2/editing=0: ganti `" a: add  x: remove"` jadi `" a: add  e: edit  x: remove  ↑↓: browse"`.
- [x] `initValueMap` di `mapping_edit_form.go`: ganti default placeholder dari `"Dest value..."` ke `"Source value..."` (karena editing=1 minta source). Set `valueMapEditIdx = -1`.

### Test (manual QA — bukti screenshot/log di PR)
- [ ] Open ENUM dest col tanpa value_map → border highlight saat tab, **TIDAK** auto-add. Tekan `a` → mode add. Ketik → terlihat di input.
- [ ] Add 2 pair → tampil di list.
- [ ] `↑↓` di browse mode → cursor pindah antar pair.
- [ ] `e` di pair kedua → input prefill source value lama → ubah → enter → pilih dest → enter → pair kedua TER-REPLACE (bukan append, jumlah pair tetap 2).
- [ ] `x` di pair → pair terhapus.
- [ ] Esc dari mid-add → tidak ada pair tambahan, idx ter-reset.
- [ ] Kolom non-ENUM → section TIDAK muncul.
- [ ] Save → pair persist di reload.

### Close-out
- [x] `gitnexus_detect_changes()` → attach di PR (hanya symbol scope).
- [ ] PR merged ke `main`.
- [ ] `bd close dbsync-e44`.
- [ ] `git push && bd dolt push && git status` clean.

---

## ☐ Step 2 — bd-14b · TUI List: enum domain mismatch warning + save guard

- **Beads:** `dbsync-tqj`
- **GitHub:** [#39](https://github.com/kentoespdam/dbsync/issues/39)
- **Branch suggest:** `feat/bd-14b-list-enum-mismatch-warning`
- **Plan section:** `bd-14b — TUI List` di plan.
- **Prerequisite:** Step 1 merged ✅.
- **Touches:**
  - `internal/storage/mappings.go` (opsi A: rename `stringSetsEqual` → `StringSetsEqual`) **ATAU** `internal/mysql/schema.go` (opsi B: tambah `EnumDomainEquals`).
  - `internal/tui/mapping_editor_view.go` (helper + extend `mappingStatus`/`statusText`/`renderHeader`).
  - `internal/tui/mapping_editor_update.go` (save guard).
  - Test setempat (storage atau mysql tergantung opsi).
- **TIDAK** menyentuh: `internal/engine/**`, `internal/cli/**`, `internal/tui/mapping_edit_form*.go`, ADR file.

### Pre-work
- [x] `gitnexus_context({name: "stringSetsEqual"})` + `gitnexus_impact({target: "stringSetsEqual"})`. Opsi A confirmed (1 caller). Risk: LOW.
- [x] `gitnexus_context({name: "mappingStatus"})` + `gitnexus_impact({target: "mappingStatus"})`. 6 direct callers, TUI only. Risk: CRITICAL (expected).
- [x] `gitnexus_context({name: "renderHeader"})` + `gitnexus_context({name: "save"})` (di mapping_editor_update). Pattern confirmed (error return).
- [x] `context7` query `github.com/charmbracelet/lipgloss` topik "Style.Foreground color codes". Catat di PR.

### Implementasi
- [x] Ekspor helper (opsi A: `StringSetsEqual` di storage + update internal call site).
- [x] Helper TUI `valueMapCoversSource(valueMap sql.NullString, srcEnum []string) bool` di `mapping_editor_view.go`.
- [x] Method `(m mappingEditorModel) enumMismatch(mp storage.Mapping, dc mysql.Column) bool` — tambah `findSourceCol` di `mapping_editor_data.go`.
- [x] Extend `mappingStatus`: ⚡ branch sebelum ✓.
- [x] Extend `statusText`: `case "⚡"`.
- [x] Extend `renderHeader` counter: `mismatch` var + `"%d ⚡ mismatch"`.
- [x] Save guard di `mapping_editor_update.go` `save()`: loop `m.mappings`, tolak kalau ada ⚡.

### Test wajib
- [x] `storage` (opsi A) — rename tidak break tests: `go test ./internal/storage/...` → ok.
- [ ] `mysql` (opsi B) — skip (opsi A dipilih).
- [ ] `tui` — tidak wajib (per CLAUDE.md). Bukti via manual QA.

### Manual QA (bukti screenshot di PR)
- [ ] Tabel mismatch (source `('Draft','Ditampilkan')`, dest `('DRAFT','PUBLISHED','DELETED')`) tanpa value_map → status `⚡` kuning, statusText benar, counter `"1 ⚡ mismatch"`.
- [ ] Edit row set value_map cover semua source → status jadi `✓`, counter mismatch turun.
- [ ] Edit row set value_map partial → status tetap `⚡`.
- [ ] Domain identik → tetap `✓` tanpa value_map.
- [ ] Non-ENUM column → status existing tidak berubah.
- [ ] `s` save saat ada `⚡` → tolak dengan pesan jelas.
- [ ] Save sukses setelah semua resolve.

### Close-out
- [x] `gitnexus_detect_changes()` → attach di PR (hanya symbol scope).
- [ ] PR merged ke `main`.
- [ ] `bd close dbsync-tqj`.
- [ ] `git push && bd dolt push && git status` clean.

---

## Final QA (post bd-14b, oleh reviewer terakhir)

- [ ] End-to-end kasus mismatch terdeteksi di list, user edit form tanpa hang, save guard menolak save saat ada `⚡`, save lolos setelah resolve, sync run sukses.
- [ ] Backward-compat: tabel tanpa enum mismatch & tanpa value_map tetap berjalan persis.
- [ ] `go test ./...` hijau.
- [ ] `go build -o dbsync ./cmd/dbsync` sukses.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan + ADR 0005.
- **Test gagal di area yang bukan scope issue** → flag di PR description, jangan tutup-tutupi.
- **`gitnexus_impact` HIGH/CRITICAL** → stop, lapor user, tunggu konfirmasi.
- **bd-14b opsi A vs B** → default opsi A; pindah B hanya kalau A bikin > 3 caller berubah.
