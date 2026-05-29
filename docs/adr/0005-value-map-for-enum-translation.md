# Value Map untuk translasi domain value source → dest

## Konteks

Kasus nyata: source kolom `enum('Draft','Ditampilkan')`, dest kolom
`enum('DRAFT','PUBLISHED','DELETED')`. Index 1 & 2 secara konseptual sama,
tapi label berbeda case dan dest punya value ekstra. Tanpa translasi, sync
gagal di runtime karena MySQL menolak value yang bukan anggota domain ENUM
dest. Pola serupa muncul di luar ENUM: status code legacy → kanonikal,
locale (`id` → `id-ID`), boolean `Y/N` → `1/0`, dll.

`storage.Mapping` saat ini hanya punya `SourceColumn` dan `DefaultValue`.
Keduanya tidak bisa menerjemahkan **value**: `SourceColumn` cuma rename
kolom, `DefaultValue` cuma fallback saat row NULL.

## Keputusan

Tambah satu field di `storage.Mapping`:

```go
ValueMap sql.NullString // JSON {"src":"dest"}
```

Migration SQLite:

```sql
ALTER TABLE mappings ADD COLUMN value_map TEXT
  CHECK (value_map IS NULL OR json_valid(value_map));
```

Semantik di `engine.Resolve`:

```
row[src] == nil → default_value (kalau ada), else nil
value_map == nil → passthrough (backward-compat)
value_map[key] hit → dest value
value_map miss → error row, log JSONL, batch lanjut, exit 1
```

Validasi storage: kalau dest kolom MySQL bertipe ENUM, semua *value* di map
harus ∈ `Column.EnumValues(dest)`. Kalau dest bukan ENUM, no check (map
generik untuk skenario non-ENUM).

CLI: `dbsync mapping set --value-map 'Draft=DRAFT,Ditampilkan=PUBLISHED'`
(shorthand) atau `--value-map-file=path.json` (mutex). `mapping auto` tidak
auto-generate value_map; ia mendeteksi mismatch domain ENUM dan menampilkan
hint command persis.

## Konsekuensi

- **Orthogonal dgn `default_value`.** `default_value` untuk row NULL,
  `value_map` untuk translasi nilai ada. Tidak digabung jadi satu field
  agar dua niat berbeda tidak saling membayangi.
- **Strict miss = error.** Konsisten dgn filosofi auditable dan
  [ADR 0003](0003-remove-mysql-error-redaction.md): silent passthrough akan
  menulis value asing ke dest dan ketahuan jauh belakangan.
- **JSON inline, bukan tabel `enum_dictionaries` separate.** Mayoritas
  mapping kecil (< 20 entry). Tabel separate menambah join + CRUD path
  tanpa benefit nyata di skala v1.
- **Backward-compat aman.** `value_map IS NULL` → passthrough lama. Semua
  mapping existing tidak perlu di-migrate datanya.

## Ditolak

- **Tabel `enum_dictionaries` reusable lintas (conn, table).** Reusability
  semu — domain enum jarang identik antar tabel; menambah indirection
  tanpa benefit.
- **Inline transform DSL (mis. expression bahasa).** Surface area besar,
  butuh parser/eval, jauh melampaui kebutuhan v1.
- **Silent passthrough saat miss.** Bertentangan dgn ADR 0003. Bug diam
  jauh lebih mahal daripada row error yang ter-log.
