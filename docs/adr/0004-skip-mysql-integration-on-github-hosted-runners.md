# ADR 0004 — Skip MySQL integration tests on GitHub-hosted runners

**Status:** Accepted
**Date:** 2026-05-29
**Decision drivers:** CI reliability on free-tier runners, PR check ergonomics, branch protection compatibility.

---

## Context

`integration.yml` menjalankan `go test -tags=integration ./...` dengan **MySQL 8.0 sebagai service container** di runner `ubuntu-latest`. Setup ini sebelumnya berjalan tapi sekarang gagal konsisten — step "Wait for MySQL" timeout (exit code 1) di run [#26612794818](https://github.com/kentoespdam/dbsync/actions/runs/26612794818) dan run-run sesudahnya.

Akar masalah: repo ini dipakai di **akun GitHub Free**. Runner gratis punya keterbatasan CPU/memory dan jadwal start service container yang tidak deterministik; MySQL 8.0 image cukup besar dan healthcheck-nya sering belum hijau dalam window 30 detik. Mahal untuk dipertahankan: setiap PR jadi merah meskipun perubahan tidak menyentuh layer MySQL.

### Constraint nyata

- **No paid plan / no self-hosted runner.** Tidak ada budget untuk Larger Runners dan tidak ada infrastruktur untuk self-hosted.
- **PR check tetap perlu hijau** supaya merge workflow lancar (operator/AI agent baca status check sebagai sinyal go/no-go).
- **Unit test sudah cukup ketat** — `crypto`, `storage`, `mysql`, `engine` semua punya unit test wajib (lihat CLAUDE.md). Integration test = extra safety net untuk perilaku terhadap MySQL nyata, bukan satu-satunya pengaman.

### Yang dipertimbangkan vs threat model

Integration test melindungi dari: charset/collation drift, MySQL version-specific behavior, real driver bugs. Risiko regresi tetap ada **tapi terdeteksi lokal** — kontributor utama menjalankan `go test -tags=integration ./...` di workstation sebelum push (workflow yang sama yang tertulis di `CLAUDE.md` § Build & Test).

---

## Decision

`integration.yml` dipertahankan namanya & jobnya (`MySQL Integration Tests`) supaya **status check tetap muncul di PR sebagai hijau**, tapi isi job-nya direduksi menjadi satu step yang `echo` pesan skip dan `exit 0`. Service container MySQL, healthcheck, setup database, dan invocation `go test` dihapus.

### Scope

1. **Hapus** `services.mysql` block.
2. **Hapus** step `Checkout`, `Setup Go`, `Wait for MySQL`, `Create test database`, `Run integration tests`.
3. **Tambah** satu step `Skip on free GitHub-hosted runner` yang menjelaskan alasan dan menunjuk ke ADR ini + perintah lokal.
4. **Pertahankan** trigger `push: main` dan `pull_request` — supaya check tetap muncul, dan supaya kalau suatu saat runner berubah, cukup revert job tanpa menyentuh trigger.
5. **Pertahankan** job name `MySQL Integration Tests` dan job id `mysql-integration` — kompatibel dengan branch protection rule yang mungkin require check ini.

### Yang TIDAK diubah

- `windows-smoke.yml`, `release.yml` — tidak berkaitan dengan service container MySQL.
- Build tag `//go:build integration` di source — test tetap ada dan tetap bisa dijalankan lokal.
- Instruksi `CLAUDE.md` § Build & Test untuk menjalankan integration test lokal.

---

## Alternatives considered

1. **Hapus `integration.yml` sepenuhnya.** Ditolak: kalau branch protection rule require check `MySQL Integration Tests`, semua PR akan ter-block. Juga menghilangkan jejak eksplisit bahwa integration test *ada* di project ini.

2. **`workflow_dispatch` only (hapus trigger PR).** Ditolak dengan alasan yang sama — PR check hilang. Kontributor harus ingat menjalankan manual. Friction tinggi.

3. **Job-level `if: false` / kondisi env.** Ditolak: GitHub menandai job sebagai *skipped* (abu-abu), bukan *success* (hijau). Beberapa branch protection setting menganggap skipped ≠ pass, jadi tetap block merge.

4. **Self-hosted runner / paid Larger Runner.** Ditolak untuk sekarang: tidak ada budget dan tidak ada infrastruktur. Pintu tetap terbuka — kalau berubah, revert job ini cukup satu commit (lihat git history).

5. **Smart retry / longer healthcheck window di workflow.** Ditolak: ini hanya menunda, tidak menyelesaikan. Runner gratis tidak menjamin resource yang konsisten — flakiness akan kembali.

---

## Trade-offs / Accepted risks

1. **Tidak ada CI gate untuk regresi MySQL real-driver di branch `main`.** Bug yang hanya muncul di kontak dengan MySQL nyata (bukan unit test) bisa lolos ke `main`. Diterima karena:
   - Kontributor menjalankan integration test lokal sebelum push (per CLAUDE.md).
   - Unit test coverage untuk `internal/mysql` sudah wajib.
   - Surface area kontak nyata didokumentasikan dan stabil (one-way sync, scrypt-encrypted creds, batch upsert).

2. **PR check yang "hijau" tidak betul-betul mem-verify apa pun di sisi MySQL.** Future reader bisa salah baca status. Mitigasi: pesan `echo` di step menyebut "skipped" eksplisit + tunjuk ke ADR.

3. **Kalau branch protection di-update dan require *output spesifik* dari step ini, akan break.** Saat ini check yang require cuma berdasarkan job name → tidak terpengaruh.

---

## Consequences

**Positif:**
- PR check stabil hijau lagi → merge tidak ter-block oleh CI noise.
- Tidak ada cost dari runner gratis yang gagal start service.
- Jejak ADR + komentar di workflow membuat keputusan ini auditable.

**Negatif:**
- Kehilangan automated MySQL regression gate. Dimitigasi via discipline lokal + unit test wajib (lihat Trade-offs §1).

**Migrasi:**
- Tidak ada perubahan kode aplikasi.
- Tidak ada perubahan `CONTEXT.md` (integration test bukan bagian dari domain language).
- Catat di `CHANGELOG.md` sebagai entry CI.

---

## References

- `CLAUDE.md` § Build & Test — perintah `go test -tags=integration ./...`
- `.github/workflows/integration.yml` — workflow yang direduksi
- GitHub Actions run yang memicu reversal: https://github.com/kentoespdam/dbsync/actions/runs/26612794818
