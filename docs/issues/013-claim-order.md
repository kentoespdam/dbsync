# Issue 013 — Claim order & progress checklist

**Parent plan:** [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md)
**ADR:** [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md)
**Tujuan:** Pastikan 5 sub-issue dikerjakan **berurutan**, satu per satu, tanpa overlap. Tiap agent claim ID berikutnya HANYA setelah PR sebelumnya merged ke `main`.

---

## ⚠️ Aturan main (baca dulu — sebelum apa-apa)

1. **Wajib baca dulu (urutan):**
   - [`CONTEXT.md`](../../CONTEXT.md) — orientasi + Glossary "Value Map".
   - [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md) — keputusan terkunci.
   - [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md) — plan detail sub-issue yang akan dikerjakan.
   - [`CLAUDE.md`](../../CLAUDE.md) — DRY comments, max 120 baris, test wajib, session close mandatory push.
2. **Skill/MCP wajib (per sub-issue):**
   - `gitnexus_impact({target, direction:"upstream"})` SEBELUM edit symbol. HIGH/CRITICAL → STOP & lapor.
   - `gitnexus_detect_changes()` SEBELUM commit. Pastikan hanya symbol scope.
   - `context7` query SEBELUM koding pakai lib eksternal yang relevan. Catat di PR description.
   - **Jangan** `ls -R` / grep buta — pakai `gitnexus_query` / `gitnexus_context`.
3. **Plan doc adalah sumber kebenaran.** Improvisasi di luar yang sudah locked = ditolak di review.
4. **Patuh dependency chain** — jangan claim issue yang dependency-nya belum closed.
5. **Satu agent = satu issue.** Jangan paralel di branch yang sama.
6. **Jangan edit file di luar `Touches` sub-issue.** Termasuk test/dokumen/CI. Kalau perlu — file issue baru.
7. **Patuh `CLAUDE.md` rules:** DRY comments (ref `bd-13X`, jangan duplikasi deskripsi issue), max 120 baris per file/function, no DI framework, no CGo.
8. **Backward-compat WAJIB.** Mapping existing tanpa `value_map` MUST tetap berjalan persis seperti sebelum bd-13. Test existing tidak boleh dimodifikasi.
9. **Workflow per issue:**
   ```bash
   bd update <bd-id> --claim
   git checkout -b feat/<bd-id>-<slug>
   # … kerja, commit kecil-kecil …
   go test ./...
   go build -o dbsync ./cmd/dbsync
   git push -u origin feat/<bd-id>-<slug>
   gh pr create --base main --title "<bd-id>: …" \
     --body "Closes #<gh-num>. Plan: docs/issues/013-value-map-enum-translation.md"
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
bd-13a (storage)  ──►  bd-13b (engine)  ──►  bd-13c (CLI)  ──►  bd-13d (auto hint)  ──►  bd-13e (TUI)
 dbsync-y0g            dbsync-tjg            dbsync-gfd        dbsync-cor              dbsync-myy
 #29                   #30                   #31               #32                     #33
```

---

## ☑ Step 1 — bd-13a · Storage: schema + Mapping.ValueMap + repo CRUD + validation

- **Beads:** `dbsync-y0g`
- **GitHub:** [#29](https://github.com/kentoespdam/dbsync/issues/29)
- **Branch suggest:** `feat/bd-13a-value-map-storage`
- **Plan section:** `bd-13a — Storage` di [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md).
- **Touches:** `internal/storage/migrations.go` (atau migrasi setara), `internal/storage/mappings.go`, `internal/storage/mappings_test.go`.
- **TIDAK** menyentuh: `internal/engine/**`, `internal/cli/**`, `internal/tui/**`, `internal/mysql/**`.

### Pre-work
- [x] Baca `CONTEXT.md` + ADR 0005 + section `bd-13a` di plan.
- [x] `gitnexus_context({name: "Mapping"})` + `gitnexus_impact({target: "Mapping", direction: "upstream"})`. Catat di PR.
- [x] `context7` query `modernc.org/sqlite` topik "ALTER TABLE ADD COLUMN, CHECK json_valid, idempotent migration". Catat di PR.

### Implementasi
- [x] Migrasi additive idempotent `ALTER TABLE mappings ADD COLUMN value_map TEXT CHECK (value_map IS NULL OR json_valid(value_map))`.
- [x] Tambah `Mapping.ValueMap sql.NullString` + update SELECT/INSERT/UPSERT konsisten.
- [x] Helper `ValidateMapping(m Mapping, destCol mysql.Column) error`: kalau `destCol.EnumValues()` non-empty, semua *value* (RHS) ∈ `EnumValues`. Else skip.
- [x] Validasi struct existing (Source/Default/ValueMap min satu) tetap berlaku.

### Test
- [x] Round-trip Insert/List dengan & tanpa `value_map`.
- [x] Upsert overwrite `value_map`.
- [x] CHECK constraint reject JSON invalid.
- [x] `ValidateMapping` 3 kasus (valid, miss, non-ENUM).
- [x] Backward-compat: existing test tetap hijau **tanpa modifikasi**.

### Close-out
- [x] `gitnexus_detect_changes()` → attach di PR (hanya symbol scope).
- [ ] PR merged ke `main`.
- [x] `bd close <bd-id>`.
- [x] `git push && bd dolt push && git status` clean.

---

## ☐ Step 2 — bd-13b · Engine: extend Resolve dengan value_map lookup

- **Beads:** `dbsync-tjg`
- **GitHub:** [#30](https://github.com/kentoespdam/dbsync/issues/30)
- **Branch suggest:** `feat/bd-13b-value-map-engine`
- **Plan section:** `bd-13b — Engine` di plan.
- **Prerequisite:** Step 1 merged ✅.
- **Touches:** `internal/engine/mapping.go`, `internal/engine/mapping_test.go`.
- **TIDAK** menyentuh: `internal/storage/**`, `internal/cli/**`, `internal/tui/**`, `internal/mysql/**`.

### Pre-work
- [ ] `gitnexus_context({name: "Resolve"})` + `gitnexus_impact({target: "Resolve"})`.
- [ ] `context7` (opsional) `encoding/json` topik unmarshal map.

### Implementasi
- [ ] Parse `m.ValueMap` sekali di `Resolve` (di luar closure).
- [ ] Tambah lookup layer di `ValueFn`: passthrough kalau map nil, hit → dest, miss → error.
- [ ] Validasi: `ValueMap` set tapi `SourceColumn` invalid → `Resolve` error.
- [ ] **Jangan** ubah signature `Resolve` tanpa alasan kuat (gitnexus_impact dulu).

### Test
- [ ] Passthrough (cover 3 case lama tetap pass).
- [ ] Hit, miss, NULL row + default + value_map, NULL row tanpa default + value_map.
- [ ] Invalid JSON `ValueMap` → error.
- [ ] `ValueMap` tanpa `SourceColumn` → error.

### Close-out
- [ ] `gitnexus_detect_changes()` → attach di PR.
- [ ] PR merged.
- [ ] `bd close <bd-id>`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 3 — bd-13c · CLI: `--value-map` + `--value-map-file` flags

- **Beads:** `dbsync-gfd`
- **GitHub:** [#31](https://github.com/kentoespdam/dbsync/issues/31)
- **Branch suggest:** `feat/bd-13c-value-map-cli`
- **Plan section:** `bd-13c — CLI` di plan.
- **Prerequisite:** Step 2 merged ✅.
- **Touches:** `internal/cli/mapping.go` (atau `mapping_set.go`), test setempat.
- **TIDAK** menyentuh: `internal/storage/**`, `internal/engine/**`, `internal/tui/**`, `internal/mysql/**`.

### Pre-work
- [ ] `gitnexus_context({name: "mappingSetCmd"})` (atau nama Cobra command terkini).
- [ ] `context7` query `github.com/spf13/cobra` topik "MarkFlagsMutuallyExclusive, custom flag parsing".

### Implementasi
- [ ] Flag `--value-map` (shorthand `k=v,k=v`) + `--value-map-file` (path JSON), mutex via `MarkFlagsMutuallyExclusive`.
- [ ] Parser shorthand: trim whitespace, error kalau key/value kosong.
- [ ] Parser file: `os.ReadFile` + `json.Unmarshal` ke `map[string]string`.
- [ ] Serialize ke JSON canonical (sorted keys) untuk determinism.
- [ ] Surface error dari `ValidateMapping` ke stderr + exit code != 0.

### Test
- [ ] Parser shorthand: berbagai bentuk, including value ber-`=`.
- [ ] Parser file: valid/invalid JSON.
- [ ] Mutex enforced oleh Cobra.
- [ ] JSON output deterministik.

### Manual QA
- [ ] `dbsync mapping set ... --value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'` → row tersimpan.
- [ ] Same via `--value-map-file`.
- [ ] Dest ENUM, value asing → error jelas, exit != 0.

### Close-out
- [ ] `gitnexus_detect_changes()` → attach di PR.
- [ ] PR merged.
- [ ] `bd close <bd-id>`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 4 — bd-13d · Auto-map: deteksi mismatch domain ENUM + hint command

- **Beads:** `dbsync-cor`
- **GitHub:** [#32](https://github.com/kentoespdam/dbsync/issues/32)
- **Branch suggest:** `feat/bd-13d-automap-enum-hint`
- **Plan section:** `bd-13d — Auto-map` di plan.
- **Prerequisite:** Step 3 merged ✅.
- **Touches:** `internal/storage/mappings.go` (extend `AutoMap`/`AutoMapResult`), `internal/storage/mappings_test.go`, `internal/cli/mapping.go` (hanya print hint baru).
- **TIDAK** menyentuh: `internal/engine/**`, `internal/tui/**`, `internal/mysql/**` (selain konsumsi `EnumValues`).

### Pre-work
- [ ] `gitnexus_context({name: "AutoMap"})` + `gitnexus_impact({target: "AutoMap"})`.

### Implementasi
- [ ] Tambah type `EnumDomainMismatch` + field `EnumMismatches` di `AutoMapResult`.
- [ ] `AutoMap` deteksi: source & dest dua-duanya ENUM, set tidak identik → append mismatch.
- [ ] **JANGAN** auto-generate `value_map` (lock ADR 0005).
- [ ] CLI handler `mapping auto`: print blok actionable dengan suggested command lengkap (`--connection=`, `--table=`, `--dest=`, `--value-map='…'`).

### Test
- [ ] Domain identik → 0 mismatch.
- [ ] Beda case → tercatat.
- [ ] Dest superset → tercatat.
- [ ] Salah satu non-ENUM → tidak masuk.
- [ ] Suggested command berisi pasangan berbasis index sebagai best-effort.

### Manual QA
- [ ] `dbsync mapping auto` di tabel mismatch nyata → output sesuai contoh di plan.

### Close-out
- [ ] `gitnexus_detect_changes()` → attach di PR.
- [ ] PR merged.
- [ ] `bd close <bd-id>`.
- [ ] `git push && bd dolt push`.

---

## ☐ Step 5 — bd-13e · TUI: value_map editor di mapping edit form

- **Beads:** `dbsync-myy`
- **GitHub:** [#33](https://github.com/kentoespdam/dbsync/issues/33)
- **Branch suggest:** `feat/bd-13e-value-map-tui`
- **Plan section:** `bd-13e — TUI` di plan.
- **Prerequisite:** Step 4 merged ✅.
- **Touches:** `internal/tui/mapping_edit_form.go` (extend), `internal/tui/mapping_editor.go` (kalau perlu indikator badge), test setempat.
- **TIDAK** menyentuh: `internal/storage/**`, `internal/engine/**`, `internal/cli/**`.

### Pre-work
- [ ] `gitnexus_context({name: "mappingEditFormModel"})` (atau nama type modal terkini).
- [ ] `gitnexus_impact({target: "mappingEditFormModel"})`. HIGH/CRITICAL → STOP & lapor.
- [ ] `context7` query `github.com/charmbracelet/bubbles/textinput`, `github.com/charmbracelet/bubbles/list`, `github.com/charmbracelet/lipgloss`.

### Implementasi (scope minimal v1)
- [ ] Section "Value Map" tampil **hanya** kalau dest column ENUM.
- [ ] Editor pair `src → dest` (textinput / list custom). Add (`enter`), remove (`x`), tab pindah field.
- [ ] Dropdown hint dest dari `Column.EnumValues(dest)`.
- [ ] Save validasi (block kalau ada value bukan anggota `EnumValues`) → toast merah (konsisten bd-09d).
- [ ] Save sukses → JSON canonical → `MappingRepo.Upsert` → toast hijau.
- [ ] Esc dengan dirty → confirm discard (existing).

### Manual QA
- [ ] Open ENUM column tanpa value_map → section kosong + info "passthrough".
- [ ] Add pair `Draft → DRAFT`, `Ditampilkan → PUBLISHED` → save → reload → row punya indikator (mis. `[map]`).
- [ ] Add pair invalid → save → toast merah, perubahan tidak persist.
- [ ] Edit existing → load terisi → ubah → save → DB ter-update.
- [ ] Esc dirty → confirm.
- [ ] Kolom non-ENUM → section TIDAK muncul.

### Close-out
- [ ] Build `go build -o dbsync ./cmd/dbsync` sukses.
- [ ] `gitnexus_detect_changes()` → attach di PR.
- [ ] PR merged.
- [ ] `bd close <bd-id>`.
- [ ] `git push && bd dolt push`.

---

## Final QA (post bd-13e, oleh reviewer terakhir)

Checklist lengkap di [`013-value-map-enum-translation.md`](./013-value-map-enum-translation.md) section "Final QA". Ringkas:

- [ ] End-to-end kasus Draft/Ditampilkan → DRAFT/PUBLISHED/DELETED sync sukses.
- [ ] Row dengan value asing → ter-log JSONL, batch lanjut, exit 1.
- [ ] TUI editor jalan, validasi block save.
- [ ] Backward-compat: tabel tanpa value_map tetap sync persis.
- [ ] `go test ./...` hijau.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan + ADR 0005.
- **Test gagal di area yang bukan scope issue** → flag di PR description, jangan tutup-tutupi.
- **`gitnexus_impact` HIGH/CRITICAL** → stop, lapor user, tunggu konfirmasi.
