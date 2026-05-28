# ADR 0002 — Cross-platform Binary Distribution & Release Strategy

**Status:** Accepted
**Date:** 2026-05-28
**Decision drivers:** portability (Linux/Windows), dev ergonomics (auto-versioning), provenance (checksums), installation simplicity (no manual build), transparency (changelog).

---

## Context

`dbsync` ditargetkan untuk operator (Linux server) dan developer (Windows laptop/WSL). Meminta user untuk melakukan `go build` sendiri memiliki beberapa hambatan:
1. **Toolchain dependency** — user harus punya Go installed dengan versi yang tepat.
2. **Version ambiguity** — binary hasil build manual seringkali kehilangan metadata versi (`v1.0.0-dev` fallback).
3. **Inconsistency** — build flags (CGO, optimization, stripping) bisa bervariasi antar user.

Dibutuhkan mekanisme rilis yang **otomatis, terstandarisasi, dan multi-platform** setiap kali ada milestone (Git tag).

---

## Decision

Menggunakan **GoReleaser v2** terintegrasi dengan **GitHub Actions** sebagai pipeline rilis utama.

### 1. Build Matrix
- **OS/Arch:** `linux/amd64` dan `windows/amd64`.
- **Format:** `.tar.gz` untuk Linux, `.zip` untuk Windows (native feel).
- **Static Linking:** `CGO_ENABLED=0` diwajibkan untuk portabilitas maksimal (tidak bergantung pada glibc version di Linux).

### 2. Version Injection
Metadata di-inject ke `main.version` via `-ldflags` saat build:
- `main.version`: `v{{ .Version }}` (dari Git tag).
- `main.commit`: `{{ .ShortCommit }}`.
- `main.date`: `{{ .Date }}`.
Ini memastikan `dbsync --version` memberikan informasi akurat tentang asal binary tersebut.

### 3. Release Artifacts
Setiap archive rilis harus menyertakan:
- Binary (`dbsync` atau `dbsync.exe`).
- `README.md` (instruksi cepat).
- `CHANGELOG.md` (transparansi perubahan).
- `LICENSE` (MIT License).
- File checksum global (`*_checksums.txt`) menggunakan SHA-256.

### 4. Automation Trigger
- **Workflow:** `.github/workflows/release.yml`.
- **Trigger:** Push tag dengan pola `v*` (misal `v1.0.0`, `v1.1.0-beta.1`).
- **Runner:** `ubuntu-latest`.

---

## Alternatives considered

1. **Manual Makefile + GitHub CLI (`gh release`).** Ditolak: terlalu banyak boilerplate manual untuk generate checksums, changelog, dan archiving. GoReleaser adalah standar de-facto di ekosistem Go.
2. **Pakai Docker images saja.** Ditolak: `dbsync` adalah tool CLI/TUI yang sering berinteraksi dengan local files dan butuh low-latency UI. Binary native lebih superior untuk pengalaman TUI (bubbletea).
3. **Ncc / Vercel PKG style.** Tidak relevan untuk Go yang sudah compile ke single binary secara native.

---

## Trade-offs / Accepted risks

1. **Windows SmartScreen / Code Signing.** Binary Windows tidak di-sign dengan sertifikat Authenticode (biaya mahal & proses ribet untuk project awal).
   - **Risiko:** Windows akan menampilkan warning "Unknown Publisher".
   - **Mitigasi:** Dokumentasikan prosedur "Run Anyway" di README (S4). Evaluasi signing di masa depan (F5) jika jumlah user Windows signifikan.

2. **Testing di Release Pipeline.** Workflow rilis saat ini hanya fokus pada build & publish.
   - **Risiko:** Binary yang rusak bisa ter-publish jika unit tests tidak dijalankan di runner yang sama.
   - **Mitigasi:** `release.yml` menjalankan `go test ./...` sebelum build. Integration tests (MySQL) dipisah ke workflow berbeda (F2) agar tidak memperlambat rilis.

3. **Matrix Terbatas (No arm64/Darwin).** Saat ini hanya amd64 Linux/Windows.
   - **Risiko:** User Mac (M1/M2) atau ARM server tidak ter-cover.
   - **Mitigasi:** amd64 Linux berjalan di ARM via emulasi (beberapa kasus), tapi native support mudah ditambah ke GoReleaser nanti (F4) jika ada request.

---

## Consequences

**Positif:**
- User bisa langsung download & run tanpa install Go.
- Versi binary sinkron dengan Git tags.
- Integrity check (checksums) tersedia untuk keamanan.

**Negatif:**
- Ada maintenance cost untuk config GoReleaser (meskipun minimal).
- GitHub Actions minutes terpakai untuk setiap tag rilis.

---

## References

- `docs/issues/011-cross-platform-release.md` → Project plan.
- [GoReleaser v2 Documentation](https://goreleaser.com/blog/goreleaser-v2/)
- `CLAUDE.md` → Coding Standards §"Shell" (cross-compile rules).
