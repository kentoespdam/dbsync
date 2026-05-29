# ADR 0003 — Remove MySQL error redaction from log output

**Status:** Accepted
**Date:** 2026-05-29
**Supersedes (partial):** [ADR 0001 §"Decision" sections 1 (redactHandler), 3 (`internal/redact/`), and `internal/logger.SanitizeError` usage]
**Decision drivers:** error actionability, debuggability of failed syncs, junior dev + AI agent ergonomy.

---

## Context

ADR 0001 (§3) memperkenalkan `internal/redact/` — helper yang strip semua **quoted values** di error MySQL sebelum ditulis ke log (`applog` via `redactHandler` dan `logger/jsonl` via `SanitizeError`). Tujuan awalnya adalah mencegah data sensitif (PII) muncul di file log.

Implementasi: regex `'[^']*'|"[^"]*"` → `'[REDACTED]'`. Regex ini **tidak membedakan** antara:

- **Value literal** (data row): `Duplicate entry 'john@example.com'` ✅ memang sensitif.
- **Identifier MySQL** (nama kolom/tabel/constraint): `for key 'users.email_idx'`, `Data truncated for column 'phone'`, `Table 'mydb.users' doesn't exist` ❌ bukan data sensitif — itu nama schema.

Akibatnya, error MySQL paling umum kehilangan informasi paling penting untuk debugging.

### Kasus konkret yang memicu reversal

Log entry dari production sync (`logger/jsonl`):

```
{"timestamp":"2026-05-29T07:34:57+07:00","level":"batch_error","batch":1,
 "error":"upsert: upsert exec: Error 1265 (01000): Data truncated for column '[REDACTED]' at row 1",
 "sql_template":"batch execution"}
```

Operator melihat error ini dan **tidak tahu kolom mana** yang harus dibetulkan size/type-nya. Padahal value yang di-redact (`'phone'`) adalah nama kolom, bukan PII. Error jadi tidak actionable.

### Threat model

dbsync dipakai untuk one-way MySQL sync di server PDAM (data customer/billing). Karakteristik log:

- File log ada di `<exeDir>/logs/` di server yang sama dengan binary.
- Konsumen log: operator/sysadmin yang sudah punya akses ke server (dan ke MySQL itu sendiri).
- Belum ada log shipping ke central system atau pihak ketiga.

Operator yang bisa baca file log = operator yang bisa akses MySQL langsung. Redaction tidak menambah security boundary real; ia hanya menambah hambatan debugging.

### Yang masih sensitif (tidak di-cover ADR ini)

- **Password connection MySQL** di error connect string — sudah di-redact terpisah di `internal/mysql/pool.go redactError` (ganti dengan `***`). Itu konsern berbeda dan **tetap berlaku**.
- **Row payload data** — `logger/jsonl` menyimpan `RowPK` (PK row gagal), bukan full row. Tidak diubah ADR ini.

---

## Decision

Hapus error-message redaction dari kedua jalur log. Error MySQL ditulis apa adanya.

### Scope

1. **Hapus** package `internal/redact/` (file `error.go` + `error_test.go`).
2. **Hapus** `internal/applog/redact_handler.go` dan unwrap base `slog.Handler` di `applog.Init`.
3. **Hapus** `SanitizeError` di `internal/logger/jsonl.go`; callsite `RowError` / `BatchError` pakai `err.Error()` langsung.
4. **Hapus** `redact.Error(err)` di `internal/tui/conn_check.go` test connection result display; tampilkan `err.Error()` langsung.
5. **Update test** yang men-assert string `[REDACTED]` agar assert raw error MySQL.

### Yang TIDAK diubah

- `internal/mysql/pool.go redactError` (password → `***`) — beda concern, tetap berlaku.
- `internal/logger/jsonl.go` Entry shape dan filename pattern.
- `applog` lumberjack rotation, `AddSource`, `DBSYNC_LOG_LEVEL`, file-only writer.
- Path `<exeDir>/logs/` lokasi log.

---

## Alternatives considered

1. **Smart redaction (preserve identifier, redact value).** Tambah parser yang membedakan `for key/column/table 'X'` (preserve) vs `Duplicate entry 'X'` (redact). Ditolak: kompleksitas tinggi (perlu maintain pattern per MySQL error code), butuh test matrix besar, melanggar prinsip "Simple > clever". Threat model tidak menjustifikasi biaya itu.

2. **Opt-out via env (`DBSYNC_LOG_REDACT=false`).** Ditolak: default tetap broken untuk operator baru; mereka harus tahu env ini ada. Lebih jujur menghapus saja dan dokumentasikan bahwa log boleh berisi error MySQL utuh.

3. **Hapus hanya di `applog`, pertahankan di `logger/jsonl`.** Ditolak: kasus yang memicu reversal justru di JSONL (`batch_error` entry); pertahanan setengah tidak menyelesaikan masalah aktual.

4. **Tetap dengan redaction lama.** Ditolak: error tidak actionable, dan threat model tidak nyata.

---

## Trade-offs / Accepted risks

1. **Log MySQL error berisi value duplicate yang mungkin PII.** Contoh: kalau ada `Duplicate entry 'customer@example.com' for key 'users.email_idx'`, email customer bocor ke `dbsync.log` / JSONL. Diterima karena:
   - File log file-only, tidak di-ship eksternal (kalau ini berubah, naikkan ADR baru).
   - Operator pembaca log = operator pemegang akses MySQL.
   - Value yang muncul = value yang collide saat upsert; debugging butuh tahu value itu.
   - Mitigasi opsional masa depan: smart redaction (Alt 1) bisa di-introduce tanpa breaking change kalau threat model berubah.

2. **`logger/jsonl` Entry shape unchanged tapi semantik field `Error` berubah** (dari sanitized → raw). Tidak ada konsumen eksternal JSONL saat ini (cuma operator manual review), jadi tidak ada migration concern. Catatan di `CHANGELOG.md` cukup.

3. **Test connection display di TUI menampilkan error MySQL utuh.** TUI list-test (`internal/tui/conn_check.go`) sekarang menampilkan `✗ Error 1045 (28000): Access denied for user 'foo'@'1.2.3.4' (using password: YES)`. Diterima — operator butuh tahu user/host mana yang gagal.

---

## Consequences

**Positif:**
- Error log actionable lagi. Operator bisa baca log dan tahu kolom/tabel/constraint yang bermasalah.
- AI Agent auto-triage punya informasi cukup untuk suggest fix.
- Hapus 1 package (`internal/redact/`) + handler wrapper → kode lebih sedikit, konsisten dengan "Simple > clever".

**Negatif:**
- Jejak PII potensial di log file kalau collide value = data customer. Mitigated by threat model (lihat Trade-offs §1).

**Migrasi (dikerjakan AI Agent lain via Issue bd-XX):**
- Hapus `internal/redact/`.
- Hapus `internal/applog/redact_handler.go`, update `applog.go` unwrap handler.
- Update `internal/logger/jsonl.go` callsite + hapus `SanitizeError`.
- Update `internal/tui/conn_check.go` callsite.
- Update test files (`applog_test.go`, `jsonl_test.go`) yang assert `[REDACTED]`.
- Update `CONTEXT.md` (§"Logging dua-jalur", §Security, §Glossary, §Layout).
- Update header ADR 0001 dengan "Superseded in part by ADR 0003".

---

## References

- `CONTEXT.md` → §"Logging dua-jalur"
- `docs/adr/0001-application-logging-strategy.md` → decision yang di-superseded sebagian
- `docs/issues/012-remove-error-redaction.md` → execution plan untuk AI Agent
- `internal/redact/error.go` → kode yang akan dihapus
- `internal/mysql/pool.go` → password redaction (tetap)
