# Issue 012 — Remove MySQL error redaction from log output

**Type:** AFK (dikerjakan AI Agent lain, manusia review)
**Triage label:** `needs-triage`, `ready-for-agent`, `refactor`
**Blocked by:** —
**Blocks:** —
**Parent ADR:** [`docs/adr/0003-remove-mysql-error-redaction.md`](../adr/0003-remove-mysql-error-redaction.md)
**Supersedes (partial):** [`docs/adr/0001-application-logging-strategy.md`](../adr/0001-application-logging-strategy.md) §Decision sub-1 (redactHandler) dan sub-3 (`internal/redact/`)
**Related:** `internal/redact/`, `internal/applog/`, `internal/logger/`, `internal/tui/conn_check.go`, `internal/mysql/pool.go` (TIDAK diubah)

---

## ⚠️ WAJIB DIBACA DULU SEBELUM CODING

Urutkan strict — jangan skip:

1. **`CLAUDE.md`** project root — rules: DRY comment, max 120 baris/file, test wajib untuk `storage`/`mysql`/`engine`, `bd` untuk task tracking, **GitNexus impact analysis WAJIB sebelum edit symbol**, **context7 WAJIB sebelum pakai lib**.
2. **`CONTEXT.md`** root — terutama §"Logging dua-jalur", §Security, §Glossary (Test Connection).
3. **`docs/adr/0003-remove-mysql-error-redaction.md`** — decision lengkap, threat model, trade-offs. **Ini ground truth.** Kalau ada pertanyaan "boleh tidak X?", jawabannya ada di ADR. Kalau ADR tidak mention, STOP dan tanya manusia.
4. **`docs/adr/0001-application-logging-strategy.md`** — konteks original logging strategy. Sub-1 (redactHandler) dan sub-3 (`internal/redact/`) di-supersede. Sisanya (lumberjack, paths, applog) **tetap berlaku**.

---

## Latar belakang

`internal/redact/` strip semua quoted value di error MySQL. Regex `'[^']*'|"[^"]*"` tidak bedakan antara value literal (PII) dan identifier (nama kolom/tabel/constraint). Hasilnya: error `Data truncated for column '[REDACTED]' at row 1` tidak actionable — operator tidak tahu kolom mana yang harus dibetulkan.

Ground truth & rationale: lihat ADR 0003.

---

## REQUIRED Pre-work — GitNexus & context7

### Step 1: GitNexus impact analysis (WAJIB)

Patuhi `CLAUDE.md` §"Always Do". Sebelum edit apa pun, jalankan:

```
gitnexus_detect_changes()
gitnexus_impact({target: "redact.Error", direction: "upstream"})
gitnexus_impact({target: "SanitizeError", direction: "upstream"})
gitnexus_impact({target: "redactHandler", direction: "upstream"})
gitnexus_context({name: "applog.Init"})
gitnexus_context({name: "logger.RowError"})
gitnexus_context({name: "logger.BatchError"})
```

Catat di PR description:
- Daftar caller masing-masing simbol.
- Risk level (LOW/MED/HIGH).
- Konfirmasi tidak ada caller di luar yang sudah tercantum di "Scope perubahan" di bawah.

Kalau impact analysis menunjukkan caller yang **TIDAK** ada di scope di bawah, STOP. Jangan auto-extend scope. Lapor ke manusia.

### Step 2: context7 (WAJIB untuk lib yang masih disentuh)

Walaupun ini delete-heavy, ada interaksi `log/slog`:

1. **`pkg.go.dev/log/slog`** — topik: "NewTextHandler, HandlerOptions, slog.SetDefault, base handler usage without wrapper".

Catat ringkasan jawaban di PR description.

Tidak perlu context7 untuk lib yang tidak diubah (lumberjack, mysql, sqlite).

---

## Scope perubahan (HARUS PERSIS — jangan tambah/kurang)

### A. Hapus file

- `internal/redact/error.go`
- `internal/redact/error_test.go`
- Direktori `internal/redact/` setelah kosong.
- `internal/applog/redact_handler.go`

### B. Edit file

#### B1. `internal/applog/applog.go`

Sebelum edit: **baca file penuh** untuk pahami struktur `Init()`. Cari baris yang membentuk handler:

```go
handler := &redactHandler{baseHandler}
```

Ganti dengan langsung pakai `baseHandler`:

```go
handler := baseHandler
```

Atau lebih bersih: hilangkan variabel `handler` perantara dan langsung pakai `baseHandler` di `slog.New(...)`. Pilih yang minimal diff.

Hapus juga import `"github.com/kentoespdam/dbsync/internal/redact"` kalau ada di file ini (cek lagi — mungkin import ada di `redact_handler.go` saja).

Update doc comment di atas `Init()` — kalimat `"with AddSource:true and redaction handler wrapping"` → `"with AddSource:true"`. Jangan ubah signature `Init()`.

#### B2. `internal/applog/applog_test.go`

Cari assertion `[REDACTED]` (sekitar baris 58-61):

```go
if !strings.Contains(sContent, "[REDACTED]") {
    t.Error("log file does not contain [REDACTED]")
}
```

Ganti dengan assertion bahwa **raw error MySQL** muncul utuh di log. Contoh: kalau test sebelumnya men-log error `errors.New("Duplicate entry 'foo' for key 'bar'")`, assertion baru:

```go
if !strings.Contains(sContent, "Duplicate entry 'foo' for key 'bar'") {
    t.Error("log file does not contain raw MySQL error")
}
```

Pastikan baca konteks test sekitar dulu — mungkin perlu sesuaikan dengan error yang sebenarnya di-emit di test setup. Test harus **tetap pass**, hanya assertion-nya yang berubah.

#### B3. `internal/logger/jsonl.go`

Hapus:
- Import `"github.com/kentoespdam/dbsync/internal/redact"`.
- Fungsi `SanitizeError` (baris ~99-100 termasuk `// Deprecated:` comment).

Edit dua callsite:

```go
// RowError
Error: SanitizeError(err),
```

Ganti jadi:

```go
Error: err.Error(),
```

(Hati-hati nil — cek apakah `err` mungkin nil di callsite. Kalau iya, guard `if err != nil`. Kalau tidak — lihat caller di engine — pakai langsung.)

Lakukan sama untuk `BatchError`.

#### B4. `internal/logger/jsonl_test.go`

Hapus fungsi `TestSanitizeError` (baris ~13-35). Update test yang assert `e1.Error == "Duplicate entry '[REDACTED]'"` (sekitar baris 56) menjadi assert raw error string yang di-pass. Baca konteks test fixture dulu.

#### B5. `internal/tui/conn_check.go`

Hapus import `"github.com/kentoespdam/dbsync/internal/redact"`.

Ganti baris 56 dan 60:

```go
return fmt.Sprintf("✗ Decryption failed: %s", redact.Error(err))
return fmt.Sprintf("✗ %s", redact.Error(err))
```

Jadi:

```go
return fmt.Sprintf("✗ Decryption failed: %s", err)
return fmt.Sprintf("✗ %s", err)
```

(`%s` pada error memanggil `err.Error()`. Tidak perlu `.Error()` eksplisit.)

Update doc comment di atas fungsi (baris ~52) yang mention "Errors are redacted (quoted values stripped) before display." — ganti dengan "Errors ditampilkan apa adanya (password connection sudah di-redact di mysql/pool.go)."

### C. Yang TIDAK boleh diubah

❌ **JANGAN sentuh** file-file ini — kalau impact analysis menyentuh mereka, lapor jangan auto-edit:

- `internal/mysql/pool.go` — `redactError` (password → `***`) **TETAP**. Beda concern.
- `internal/mysql/pool_test.go` — test redactError password **TETAP**.
- `internal/logger/jsonl.go` Entry struct, filename pattern, `New()`, `Close()`, `Path()`, `log()`.
- `internal/applog/applog.go` selain bagian handler wrapper (lumberjack config, level resolution, paths, AddSource, ReplaceAttr — **TETAP**).
- `internal/applog/applog_test.go` test cases lain selain assertion `[REDACTED]`.
- `internal/paths/` (tidak terkait).
- Semua file lain di `cmd/`, `internal/cli/`, `internal/config/`, `internal/crypto/`, `internal/engine/`, `internal/mysql/` (selain pool.go yang TETAP), `internal/storage/`.

### D. Dokumentasi (HARUS update — sudah ada draftnya)

Empat file dokumentasi sudah di-update oleh manusia di session sebelumnya. **Verifikasi** mereka konsisten dengan perubahan kode:

- `docs/adr/0003-remove-mysql-error-redaction.md` — sudah ada, jangan ubah kecuali ada typo.
- `docs/adr/0001-application-logging-strategy.md` — header sudah ditambah note "superseded in part". Jangan ubah.
- `CONTEXT.md` — §Logging dua-jalur, §Security, §Glossary, §Layout, §Pointer sudah di-update. Jangan ubah lagi.
- `docs/issues/012-remove-error-redaction.md` — file ini sendiri.

Kalau ada inkonsistensi yang ketemu saat coding (misal CONTEXT.md masih mention `redact/` di tempat lain), update di-flag di PR description, jangan diam-diam edit.

### E. CHANGELOG

Tambahkan entry di `CHANGELOG.md` di bawah heading versi yang sedang berjalan (cek file dulu untuk format):

```
- Removed MySQL error redaction from app log and per-sync JSONL log. Errors are now written verbatim for actionability. Connection password redaction in `mysql/pool.go` is unchanged. See ADR 0003.
```

---

## Workflow eksekusi (urutan strict)

1. `bd update <id> --claim` — claim issue.
2. Baca semua dokumen di §"WAJIB DIBACA DULU".
3. Jalankan GitNexus impact analysis (§REQUIRED Step 1). Catat hasil.
4. Jalankan context7 query (§REQUIRED Step 2). Catat ringkasan.
5. Apply scope §A (hapus file) → `go build ./...` (akan break — itu expected).
6. Apply scope §B file per file. Setelah tiap file: `go build ./...`. Jangan lanjut ke file berikutnya kalau build belum hijau.
7. `go test ./...` — semua harus pass. Kalau ada test fail di luar yang kamu ubah (B2, B4), STOP — itu sinyal scope creep tak terdeteksi. Lapor manusia.
8. `go test -tags=integration ./...` — opsional, hanya kalau Docker tersedia. Tidak boleh regress.
9. Verifikasi tidak ada referensi `redact` tersisa:
   ```bash
   grep -rn "redact\|REDACTED\|SanitizeError" --include="*.go" .
   ```
   Hasil yang **boleh** muncul: `internal/mysql/pool.go` (`redactError`, password), `internal/mysql/pool_test.go`. Selain itu = bug, harus dibersihkan.
10. `gitnexus_detect_changes()` — konfirmasi scope berubah sesuai harapan. Kalau ada simbol di luar scope, STOP.
11. Update CHANGELOG (§E).
12. `git add -A && git commit -m "refactor(log): remove MySQL error redaction (ADR 0003)"`.
13. `git push` ke branch issue.
14. Buka PR. PR description harus berisi:
    - Link ke ADR 0003 dan issue ini.
    - Hasil GitNexus impact analysis (§REQUIRED Step 1).
    - Ringkasan context7 query (§REQUIRED Step 2).
    - `git diff --stat` ringkasan.
    - Output `grep -rn "redact"` sisa (untuk audit reviewer).
    - Konfirmasi `go test ./...` hijau.
15. `bd close <id>` setelah PR merged.

---

## Acceptance criteria

- [ ] Package `internal/redact/` terhapus total.
- [ ] `internal/applog/redact_handler.go` terhapus.
- [ ] `applog.Init()` tidak lagi wrap handler dengan `redactHandler`; output `dbsync.log` berisi error MySQL apa adanya (verifikasi via test).
- [ ] `logger/jsonl.go` callsite `RowError` & `BatchError` pakai `err.Error()` langsung; `SanitizeError` terhapus.
- [ ] `tui/conn_check.go` tampilkan error MySQL apa adanya (password tetap di-redact di pool.go).
- [ ] `internal/mysql/pool.go redactError` TIDAK diubah — verifikasi via `git diff` kosong di file ini.
- [ ] `go test ./...` hijau.
- [ ] `grep -rn "redact\|REDACTED\|SanitizeError" --include="*.go" .` hanya match di `mysql/pool*.go`.
- [ ] `gitnexus_detect_changes()` melaporkan perubahan hanya di simbol/file dalam §B.
- [ ] CHANGELOG.md updated.
- [ ] PR description berisi hasil impact analysis + ringkasan context7.

---

## Definisi "selesai" (Definition of Done)

- PR di-review manusia, di-merge ke main.
- `bd close <id>` dijalankan.
- `git pull --rebase && bd dolt push && git push && git status` (Session close protocol di CLAUDE.md).

---

## Hal yang sering AI Agent salah (anti-pattern)

1. ❌ **Auto-extend scope.** Kalau impact analysis menunjukkan caller `redact.Error` di luar 5 file di §B, jangan langsung edit. Lapor manusia — mungkin ada call site baru yang ditambah setelah brief ini ditulis.
2. ❌ **Hapus juga `mysql/pool.go redactError`.** TIDAK. Itu password redaction, beda concern, TETAP.
3. ❌ **Tambah TODO comment "kalau threat model berubah, restore redaction".** TIDAK — ADR sudah jadi jejak; comment di kode = noise (lihat CLAUDE.md §"DRY comments").
4. ❌ **Refactor logger/jsonl.go di luar scope** (misal rename field, ganti format). TIDAK — issue ini single-concern.
5. ❌ **Skip GitNexus / context7** karena "ini cuma delete". TIDAK — `CLAUDE.md` `MUST` rule. Tetap jalankan, hasil short OK.
6. ❌ **Lupa update CHANGELOG.** Itu user-visible change.
7. ❌ **Commit message generic** seperti "fix logging". Pakai: `refactor(log): remove MySQL error redaction (ADR 0003)`.

---

## Referensi cepat

| Hal | Lokasi |
|---|---|
| Decision & rationale | `docs/adr/0003-remove-mysql-error-redaction.md` |
| Logging strategy original (sebagian masih berlaku) | `docs/adr/0001-application-logging-strategy.md` |
| Project rules | `CLAUDE.md` |
| Kosa kata | `CONTEXT.md` |
| Password redaction (TETAP) | `internal/mysql/pool.go redactError` |
| File yang dihapus | `internal/redact/`, `internal/applog/redact_handler.go` |
| Callsite di-edit | `internal/applog/applog.go`, `internal/logger/jsonl.go`, `internal/tui/conn_check.go` |
| Test di-edit | `internal/applog/applog_test.go`, `internal/logger/jsonl_test.go` |
