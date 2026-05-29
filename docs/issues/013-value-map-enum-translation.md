# Issue 013 — Value Map untuk translasi ENUM (dan domain value lainnya)

**Parent:** [`docs/PRD-v1.md`](../PRD-v1.md) · [`CONTEXT.md`](../../CONTEXT.md)
**ADR:** [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md)
**Claim order:** [`013-claim-order.md`](./013-claim-order.md)

---

## ⚠️ Wajib dibaca dulu (sebelum koding)

1. [`CONTEXT.md`](../../CONTEXT.md) — orientasi proyek. Section **Glossary → "Value Map"** adalah definisi kanonik.
2. [`docs/adr/0005-value-map-for-enum-translation.md`](../adr/0005-value-map-for-enum-translation.md) — 8 keputusan terkunci dan rejected alternatives.
3. [`CLAUDE.md`](../../CLAUDE.md) — aturan agent (DRY comments, max 120 baris, test wajib, no DI, no CGo, session close mandatory push).
4. [`docs/issues/003-column-mapping-crud.md`](./003-column-mapping-crud.md) — desain awal column mapping CRUD (foundation yang di-extend issue ini).

**Skill/MCP wajib dipakai SEBELUM koding:**

- `context7` → query lib eksternal yang di-touch sub-issue (`modernc.org/sqlite`, `go-sql-driver/mysql`, `github.com/spf13/cobra`, `github.com/charmbracelet/bubbles/textinput`, dst). Catat hasil di PR description.
- `gitnexus` → `gitnexus_impact({target, direction:"upstream"})` SEBELUM edit symbol; `gitnexus_detect_changes()` SEBELUM commit. **HIGH/CRITICAL risk → stop & lapor user**.
- Untuk eksplorasi cepat: `gitnexus_query({query})` / `gitnexus_context({name})`. **Jangan** `ls -R` / grep buta.

---

## Aturan main (NON-NEGOTIABLE)

- **Satu agent = satu issue.** Jangan paralel di branch yang sama.
- **Jangan keluar plan.** Touches per issue sudah eksplisit. Kalau menemukan kebutuhan di luar scope → file issue baru, **jangan** improvisasi.
- **Jangan edit file di luar `Touches` issue.** Termasuk test, dokumen, CI. Kalau ragu → tanya di GH issue, jangan tebak.
- **Backward-compat.** `value_map IS NULL` MUST tetap perilaku lama (passthrough). Existing tests harus tetap hijau tanpa modifikasi.
- **DRY comments.** Reference `bd-13X` di kode, jangan duplikasi deskripsi issue.
- **Test wajib** untuk perubahan di `storage` / `engine`. Manual QA untuk `cli` / `tui`.
- **Patuh dependency chain** — jangan claim issue yang dependency-nya belum closed (lihat `013-claim-order.md`).
- **Session close MANDATORY**: `git pull --rebase && bd dolt push && git push && git status` harus return clean & up-to-date.

---

## Konteks masalah

Source kolom `enum('Draft','Ditampilkan')`, dest kolom `enum('DRAFT','PUBLISHED','DELETED')`.
Index 1 & 2 secara konseptual sama, tapi label beda case dan dest punya value ekstra.
Tanpa translasi, MySQL menolak insert karena value bukan anggota domain ENUM dest.
Pola serupa muncul untuk: status legacy → kanonikal, locale (`id` → `id-ID`), boolean `Y/N` → `1/0`.

`storage.Mapping` saat ini hanya punya `SourceColumn` (rename) dan `DefaultValue` (fallback row NULL).
Keduanya tidak bisa menerjemahkan **value**.

## Solusi (locked, lihat ADR 0005)

Tambah field `ValueMap sql.NullString` (JSON `{"src":"dest"}`) di `storage.Mapping`.
Engine:

```
row[src] == nil → default_value (kalau ada), else nil
value_map == nil → passthrough (backward-compat)
value_map[key] hit → dest value
value_map miss → error row, log JSONL, batch lanjut, exit 1
```

Validasi storage: kalau dest kolom MySQL bertipe ENUM, semua *value* di map
harus ∈ `Column.EnumValues(dest)`. Kalau dest bukan ENUM, no check.

CLI shorthand: `--value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'`
atau file: `--value-map-file=path.json` (mutex).

`mapping auto` **tidak** auto-generate value_map; ia mendeteksi mismatch
domain ENUM dan menampilkan hint command persis.

---

## Pecah sub-issue

```
bd-13a (storage)  ──►  bd-13b (engine)  ──►  bd-13c (CLI flags)  ──►  bd-13d (auto hint)  ──►  bd-13e (TUI editor)
```

Chain linear: tiap sub-issue **HARUS** menunggu yang sebelumnya merged ke `main`.

---

## bd-13a — Storage: schema + Mapping.ValueMap + repo CRUD + validation

**Touches:**
- `internal/storage/migrations.go` (atau file migrasi setara — cek `storage/db.go`)
- `internal/storage/mappings.go`
- `internal/storage/mappings_test.go`

**TIDAK** mengubah: `internal/engine/**`, `internal/cli/**`, `internal/tui/**`, `internal/mysql/**`.

### Pre-work
- [ ] `gitnexus_context({name: "Mapping"})` → catat semua caller di PR.
- [ ] `gitnexus_impact({target: "Mapping", direction: "upstream"})` → lapor risk level.
- [ ] `context7` query `modernc.org/sqlite` topik "ALTER TABLE ADD COLUMN, CHECK json_valid, idempotent migration". Catat di PR.

### Implementasi
- [ ] Migrasi additive (idempotent — re-run safe):
  ```sql
  ALTER TABLE mappings ADD COLUMN value_map TEXT
    CHECK (value_map IS NULL OR json_valid(value_map));
  ```
  Pakai pattern existing di `storage/` (cek file migrasi yang ada; jangan bikin sistem migrasi baru).
- [ ] Tambah field di struct:
  ```go
  type Mapping struct {
      ID           int64
      ConnectionID int64
      TableName    string
      SourceColumn sql.NullString
      DestColumn   string
      DefaultValue sql.NullString
      ValueMap     sql.NullString // JSON {"src":"dest"}, bd-13a
      CreatedAt    time.Time
  }
  ```
- [ ] Update SELECT/INSERT/UPSERT di `MappingRepo` agar baca/tulis kolom `value_map`. Urutan kolom konsisten dengan struct.
- [ ] Validasi storage **sebelum Insert/Upsert**:
  1. Parse `ValueMap.String` sebagai `map[string]string`. Error → return error jelas.
  2. Kalau caller passing `destColumn mysql.Column` (lewat helper opsional baru `ValidateMapping(m, destCol mysql.Column)`), dan `destCol.EnumValues()` non-empty, semua *value* (RHS) map HARUS ∈ `EnumValues`. Miss → return error `"value_map: value %q not in dest ENUM domain %v"`.
  3. Kalau dest bukan ENUM (atau caller tidak passing column metadata), skip check ini.
- [ ] Validasi struct existing (Source XOR Default XOR ValueMap minimal salah satu) tetap berlaku. Mapping kosong total = tetap error.

### Test (`mappings_test.go`)
- [ ] Insert + ListByTable round-trip dengan `value_map` set dan dengan `value_map` NULL.
- [ ] Upsert overwrite `value_map`.
- [ ] Insert dengan `value_map` JSON invalid → CHECK constraint reject (driver error).
- [ ] `ValidateMapping`: dest ENUM, semua value valid → OK.
- [ ] `ValidateMapping`: dest ENUM, ada value bukan anggota EnumValues → error berisi nama value & domain.
- [ ] `ValidateMapping`: dest bukan ENUM, value apapun → OK (no check).
- [ ] Backward-compat: existing test (mapping tanpa value_map) tetap hijau **tanpa modifikasi**.

### Acceptance
- [ ] `go test ./internal/storage/...` hijau.
- [ ] `gitnexus_detect_changes()` hanya menunjukkan symbol di scope (Mapping, MappingRepo, ValidateMapping, migrate). Lapor di PR.
- [ ] PR description berisi: hasil `context7`, hasil `gitnexus_impact`, alasan kalau ada deviasi dari plan.

---

## bd-13b — Engine: extend Resolve dengan value_map lookup

**Touches:**
- `internal/engine/mapping.go`
- `internal/engine/mapping_test.go` (buat jika belum ada)

**TIDAK** mengubah: `internal/storage/**`, `internal/cli/**`, `internal/tui/**`.

**Prerequisite:** bd-13a merged ke `main`.

### Pre-work
- [ ] `gitnexus_context({name: "Resolve"})` + `gitnexus_impact({target: "Resolve", direction: "upstream"})`.
- [ ] `context7` query `encoding/json` topik "Unmarshal into map[string]string with error context" (opsional kalau perlu).

### Implementasi
- [ ] Parse `m.ValueMap.String` sekali di `Resolve` (di luar closure), cache `map[string]string`. Parse error → return error dari `Resolve` (mapping invalid stop sync).
- [ ] Tambah case baru di `Resolve`:
  ```
  ValueFn(row):
    raw := row[src]
    if raw == nil:
      if defaultVal set: return defaultVal
      return nil
    if valueMap == nil:
      return raw           // backward-compat passthrough
    key := fmt.Sprint(raw)
    if v, ok := valueMap[key]; ok: return v
    return fmt.Errorf("unmapped value %q for column %s", key, dest)
  ```
  Jangan duplikasi cabang existing — refactor agar 3 cabang existing (source-only, default-only, both) tetap jalan, value_map jadi orthogonal layer di atas raw.
- [ ] `value_map` hanya valid kalau `SourceColumn.Valid` (tidak ada sumber → tidak ada yang di-translate). Validasi di awal `Resolve`, return error kalau melanggar.
- [ ] **DO NOT** ubah signature `Resolve` selain return error (kalau memang perlu). Kalau perlu return error, update caller minimal (cek `gitnexus_context` siapa caller).

### Test (`mapping_test.go`)
- [ ] Passthrough: `ValueMap` NULL → behavior identik kasus existing (cover 3 case lama).
- [ ] Hit: `ValueMap = {"Draft":"DRAFT","Ditampilkan":"PUBLISHED"}`, row `{x:"Draft"}` → `"DRAFT"`.
- [ ] Miss: row `{x:"Other"}` → error berisi `"Other"` dan nama dest column.
- [ ] NULL row + default + value_map: row NULL → default (value_map tidak konsultasi).
- [ ] NULL row tanpa default + value_map: row NULL → nil (value_map tidak konsultasi).
- [ ] `ValueMap` set tapi `SourceColumn` invalid → `Resolve` return error.
- [ ] Invalid JSON di `ValueMap.String` → `Resolve` return error.

### Acceptance
- [ ] `go test ./internal/engine/...` hijau.
- [ ] Backward-compat: test storage/engine existing tetap hijau **tanpa modifikasi**.
- [ ] `gitnexus_detect_changes()` di-attach di PR.
- [ ] Tidak menyentuh JSONL logger di issue ini (caller di runner yang nanti panggil `ValueFn` bertanggung jawab — sudah pattern existing).

---

## bd-13c — CLI: `--value-map` + `--value-map-file` flags

**Touches:**
- `internal/cli/mapping.go` (atau file `mapping_set.go` kalau sudah dipecah — cek struktur existing dulu).
- `internal/cli/mapping_test.go` (kalau ada — atau buat baru untuk parser shorthand).

**TIDAK** mengubah: `internal/storage/**`, `internal/engine/**`, `internal/tui/**`, `internal/mysql/**`.

**Prerequisite:** bd-13b merged ke `main`.

### Pre-work
- [ ] `gitnexus_context({name: "mappingSetCmd"})` (atau nama Cobra command setara). Catat existing flag handler di PR.
- [ ] `context7` query `github.com/spf13/cobra` topik "MarkFlagsMutuallyExclusive, ValidArgsFunction, custom flag parsing".

### Implementasi
- [ ] Tambah 2 flag di `mapping set`:
  - `--value-map string` — shorthand `k=v,k=v,...`. Whitespace di-trim. Empty key/value → error.
  - `--value-map-file string` — path ke file JSON `{"k":"v",...}`.
- [ ] Mutual-exclusive: pakai `cmd.MarkFlagsMutuallyExclusive("value-map", "value-map-file")`.
- [ ] Parser:
  - Shorthand: split `,`, lalu split `=` (first occurrence; value boleh mengandung `=` lain).
  - File: `os.ReadFile`, `json.Unmarshal` ke `map[string]string`. Error path/format → error jelas.
- [ ] Serialize hasil ke JSON canonical (sorted key untuk determinism) → simpan ke `Mapping.ValueMap`.
- [ ] Hand-off ke `MappingRepo.Upsert`. Validasi domain ENUM ditangani storage layer (bd-13a `ValidateMapping`), CLI tinggal panggil dan surface error.
- [ ] Help text contoh:
  ```
  --value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'
  --value-map-file ./status-map.json
  ```
- [ ] Kalau user tidak lewat `--value-map*`, jangan ubah `value_map` existing di DB (preserve on partial update via Upsert — pertimbangkan: idealnya Upsert sekarang full-overwrite; konsultasi plan, jangan ubah perilaku Upsert kalau breaking).

### Test
- [ ] Parser shorthand: 1 pair, N pair, value mengandung `=`, whitespace dipangkas.
- [ ] Parser shorthand: empty key → error; duplicate key → error (last wins ATAU error — pilih satu, dokumentasikan di help).
- [ ] Parser file: valid JSON → map; invalid JSON → error.
- [ ] Mutex flag: kedua flag dipakai → Cobra reject.
- [ ] JSON serialize deterministik (sorted keys) — assert string output.

### Manual QA
- [ ] `dbsync mapping set --connection=demo --table=articles --dest=status --source=status --value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'` → row mapping ada di SQLite, `value_map` JSON valid.
- [ ] Same dengan `--value-map-file` → identik hasilnya.
- [ ] Dest ENUM, value asing → error muncul di stderr, exit code != 0.

### Acceptance
- [ ] `go test ./internal/cli/...` hijau.
- [ ] PR description berisi hasil `context7` Cobra + screenshot/log manual QA.

---

## bd-13d — Auto-map: deteksi mismatch domain ENUM + hint command

**Touches:**
- `internal/storage/mappings.go` (extend `AutoMap` / `AutoMapResult`)
- `internal/storage/mappings_test.go`
- `internal/cli/mapping.go` (cetak hint baru di handler `mapping auto`)

**TIDAK** mengubah: `internal/engine/**`, `internal/tui/**`, `internal/mysql/**` (selain konsumsi `EnumValues`).

**Prerequisite:** bd-13c merged ke `main`.

### Pre-work
- [x] `gitnexus_context({name: "AutoMap"})` + `gitnexus_impact({target: "AutoMap"})`.
- [x] Tidak perlu `context7` (pure Go). Sebutkan di PR.

### Implementasi
- [x] Extend `AutoMapResult`:
  ```go
  type EnumDomainMismatch struct {
      DestColumn   string
      SourceValues []string // dari source ENUM
      DestValues   []string // dari dest ENUM
      Suggested    string   // command line siap-copy
  }

  type AutoMapResult struct {
      Mappings        []Mapping
      Warnings        []string
      UnmappedSource  []string
      EnumMismatches  []EnumDomainMismatch // bd-13d
  }
  ```
- [x] Logika di `AutoMap`: untuk tiap mapping di mana source & dest dua-duanya ENUM dan `EnumValues` set, bandingkan set source vs dest. Kalau **tidak identik** (set equality), append `EnumDomainMismatch`.
- [x] `Suggested` command (set saat AutoMap belum tahu connection name → biarkan placeholder `<CONN>`, atau caller di CLI yang substitusi). Pilih opsi B: caller substitusi nama connection sebelum print.
- [x] `AutoMap` tetap **TIDAK** auto-generate `value_map` (locked di ADR 0005). Hanya laporan.
- [x] CLI `mapping auto`: setelah print warnings existing, kalau `EnumMismatches` non-empty, print blok actionable:
  ```
  ⚠ 1 dest column has ENUM domain mismatch with source:
    - articles.status:
        source: [Draft, Ditampilkan]
        dest:   [DRAFT, PUBLISHED, DELETED]
        Run: dbsync mapping set --connection=demo --table=articles --dest=status \
             --value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'
  ```

### Test
- [x] Identik domain (case sama) → tidak ada mismatch.
- [x] Beda case (`Draft` vs `DRAFT`) → mismatch tercatat.
- [x] Dest superset (`Draft` ⊂ `DRAFT,PUBLISHED,DELETED`) → mismatch tercatat (karena set tidak sama).
- [x] Salah satu kolom bukan ENUM → tidak masuk `EnumMismatches`.
- [x] Pesan suggested command berisi nama tabel, dest column, dan pasangan `src=dst` kandidat berdasarkan **index** ENUM (sebagai best-effort hint; agen tidak claim ini benar, hanya saran).

### Manual QA
- [ ] Jalankan `dbsync mapping auto` pada tabel dengan ENUM mismatch nyata → output sesuai contoh di Implementasi.

### Acceptance
- [x] `go test ./internal/storage/... ./internal/cli/...` hijau.
- [ ] PR description tegaskan: AutoMap **tidak** menulis `value_map` (sesuai ADR 0005).

---

## bd-13e — TUI: value_map editor di mapping edit form

**Touches:**
- `internal/tui/mapping_edit_form.go` (atau file modal mapping edit terkini — cek dengan `gitnexus_query`).
- `internal/tui/mapping_editor.go` (mungkin perlu tahu state value_map untuk preview di list).
- File test TUI (manual QA dominan, unit test seperlunya).

**TIDAK** mengubah: `internal/storage/**`, `internal/engine/**`, `internal/cli/**`.

**Prerequisite:** bd-13d merged ke `main`.

### Pre-work
- [ ] `gitnexus_context({name: "mappingEditFormModel"})` (atau nama type modal terkini).
- [ ] `gitnexus_impact({target: "mappingEditFormModel"})`. HIGH/CRITICAL → stop & lapor.
- [ ] `context7` query `github.com/charmbracelet/bubbles/textinput` (multi-textinput, validation), `github.com/charmbracelet/bubbles/list` (kalau pakai list untuk pair editor), `github.com/charmbracelet/lipgloss` (border focus state).

### Implementasi (UX awal — minimal)
- [ ] Kalau dest column ENUM, tampilkan section "Value Map" di modal: list pair `src → dest`. Kosong = passthrough (info text).
- [ ] Editor pair: enter untuk tambah baris, `x` untuk hapus baris fokus, `tab` pindah field.
- [ ] Dropdown / hint untuk nilai dest: dari `Column.EnumValues(dest)`.
- [ ] Validasi sebelum save (consistent dengan `ValidateMapping`): semua value (RHS) HARUS ∈ `EnumValues`. Tidak valid → hard-block save (toast merah, konsisten dgn bd-09d).
- [ ] Save → serialize ke JSON canonical (sorted keys), set `Mapping.ValueMap.String`, call `MappingRepo.Upsert`.
- [ ] Kalau dest bukan ENUM, **jangan** tampilkan section value_map di v1 (scope terbatas — file follow-up issue kalau perlu generic editor).

### Manual QA
- [ ] Buka modal di kolom ENUM tanpa value_map → section kosong + info "passthrough".
- [ ] Tambah pair `Draft → DRAFT`, `Ditampilkan → PUBLISHED` → save → toast hijau → reload mapping list → row punya indikator value_map (mis. badge `[map]`).
- [ ] Tambah pair dengan dest value bukan di `EnumValues` → save → toast merah, perubahan tidak persist.
- [ ] Edit existing value_map → load awal sudah terisi → ubah → save → JSON di DB ter-update.
- [ ] Esc dengan dirty → confirm discard (existing behavior tetap).
- [ ] Kolom bukan ENUM → section value_map TIDAK muncul.

### Acceptance
- [ ] Build `go build -o dbsync ./cmd/dbsync` sukses.
- [ ] Manual QA checklist semua centang. Screenshot/recording di PR description.
- [ ] PR description: hasil `context7` bubbles, hasil `gitnexus_impact`.

---

## Final QA (post bd-13e, oleh reviewer terakhir)

- [ ] End-to-end: setup tabel dengan ENUM mismatch (kasus Draft/Ditampilkan → DRAFT/PUBLISHED/DELETED).
- [ ] `dbsync mapping auto` → laporan mismatch + suggested command muncul.
- [ ] `dbsync mapping set --value-map …` → tersimpan.
- [ ] `dbsync run --connection=X --table=Y` → sync sukses untuk row dengan value yang ada di map; row dengan value asing → ter-log di JSONL `<exeDir>/logs/sync-…jsonl`, batch lanjut, exit 1.
- [ ] TUI: edit mapping → value_map editor jalan, validasi block save.
- [ ] Backward-compat: tabel TANPA value_map tetap sync persis seperti sebelum bd-13.
- [ ] `go test ./...` hijau.

---

## Kalau ragu

- **Plan ambigu / kontradiktif** → comment di GH issue, tunggu klarifikasi. JANGAN tebak.
- **Scope kelihatan lebih besar dari plan** → kemungkinan over-improvisasi. Stop, baca ulang plan + ADR 0005.
- **Test gagal di area yang bukan scope issue** → flag di PR description, jangan tutup-tutupi.
- **`gitnexus_impact` HIGH/CRITICAL** → stop, lapor user, tunggu konfirmasi.
