# Issue 011 — Claim order & progress checklist

**Parent plan:** [`011-cross-platform-release.md`](./011-cross-platform-release.md)
**Tujuan:** Pastikan 6 main slice (S1–S6) dikerjakan **berurutan**, satu per satu, tanpa overlap. Tiap agent claim ID berikutnya HANYA setelah PR sebelumnya merged ke `main`. Follow-up F1–F5 paralel-aman, boleh dikerjakan kapan saja (atau di-defer).

---

## ⚠️ Aturan main (baca dulu)

1. **Plan doc adalah sumber kebenaran.** Buka [`011-cross-platform-release.md`](./011-cross-platform-release.md) **sebelum** menulis kode. Jangan improvisasi di luar yang sudah locked di sana.
2. **Patuh dependency chain** — jangan claim issue yang dependency-nya belum closed.
3. **Satu agent = satu slice.** Jangan paralel di branch yang sama.
4. **Patuhi `CLAUDE.md`** project rules: DRY comments, max 120 baris per file/function, `context7` wajib untuk lib eksternal (GoReleaser v2 schema).
5. **GoReleaser v2 only.** Syntax berubah dari v1 — jangan pakai `format:` (singular, deprecated), pakai `formats:` (plural).
6. **`cmd/dbsync/main.go:27` jangan diubah.** `var version = "v1.0.0-dev"` adalah intentional dev fallback, di-override via ldflags saat release.
7. **Workflow per slice** (contoh untuk S1 = `dbsync-28b`, GH `#19`):
   ```
   bd update dbsync-28b --claim
   git checkout -b feat/dbsync-28b-gitignore-dist
   # … kerja, commit kecil-kecil …
   make release-check   # mulai dari S3 onward
   go test ./...
   go build -o dbsync ./cmd/dbsync
   git push -u origin feat/dbsync-28b-gitignore-dist
   gh pr create --base main --title "dbsync-28b: chore: ignore dist/ directory" \
     --body "Closes #19. Plan: docs/issues/011-cross-platform-release.md"
   # … review + merge …
   bd close dbsync-28b
   ```
8. **Session close MANDATORY** (lihat `CLAUDE.md` "Session Completion"): `git pull --rebase && bd dolt push && git push && git status`.

---

## Claim order (linear chain)

```
S1 (.gitignore)  ──►  S2 (Makefile cross-compile)  ──►  S3 (GoReleaser + workflow)  ──►  S4 (docs: ADR + README + CONTEXT)
                                                                                              │
                                                              S5 (LICENSE — HITL) ─ paralel ──┤
                                                                                              ▼
                                                                                    S6 (cut v1.0.0 — HITL)
```

Follow-ups (paralel-aman, boleh deferred):

```
F1 (module path rename)   — independent
F2 (integration test CI)  — depends on S3
F3 (Windows smoke CI)     — depends on S3
F4 (arm64/darwin matrix)  — depends on S3, speculative
F5 (Authenticode)         — depends on S3, HITL, gated by user complaints
```

---

## ☑ Step 1 — S1 · `chore: ignore dist/ directory` ✅ DONE

**bd:** `dbsync-28b`
**GH:** [`#19`](https://github.com/kentoespdam/dbsync/issues/19)
**Type:** AFK
**Blocks:** S2

### What

Append `dist/` ke `.gitignore`. Itu saja. GoReleaser output + `make build-linux/build-windows` semua tulis ke `dist/`.

### Acceptance

- [x] `.gitignore` punya baris `dist/`.
- [x] `mkdir -p dist && touch dist/test && git status` tidak menampilkan `dist/` sebagai untracked.

---

## ☑ Step 2 — S2 · `build: add cross-compile + version-injection Makefile targets` ✅ DONE

**bd:** `dbsync-2bb`
**GH:** [`#18`](https://github.com/kentoespdam/dbsync/issues/18)
**Type:** AFK
**Blocked by:** S1
**Blocks:** S3

### What

Tambah variabel `VERSION`/`LDFLAGS`/`GOFLAGS` di Makefile (lihat plan §"Makefile additions"). Tambah target `build-linux`, `build-windows`, `build-all`, `snapshot`, `release-check`. Update target `build` agar pakai ldflags yang sama (konsistensi dev ↔ release). Update `clean` untuk hapus `dist/`. Update `help` dengan 4 line baru. Update `.PHONY`.

### Acceptance

- [x] `make build-all` produces `dist/dbsync_linux_amd64` (ELF 64-bit) + `dist/dbsync_windows_amd64.exe` (PE32+) — verify dengan `file`.
- [x] `./dist/dbsync_linux_amd64 --version` (atau `dbsync version`) print versi yang berasal dari `git describe`, BUKAN `v1.0.0-dev`.
- [x] `make build` (no GOOS override) masih jalan & inject versi yang sama.
- [x] `make clean` hapus `dbsync` binary + `dist/` directory.
- [x] `make help` cantumkan 5 target baru.

---

## ☑ Step 3 — S3 · `build: add GoReleaser config + tag-triggered release workflow` ✅ DONE

**bd:** `dbsync-w5a`
**GH:** [`#17`](https://github.com/kentoespdam/dbsync/issues/17)
**Type:** AFK
**Blocked by:** S2
**Blocks:** S4, S6

### What

Buat `.goreleaser.yml` dan `.github/workflows/release.yml` persis seperti di plan §"Konten file kunci". GoReleaser v2 syntax (`version: 2`, `formats:` plural). Workflow trigger `tags: ["v*"]` saja, runner `ubuntu-latest`, permissions `contents: write`.

### Acceptance

- [x] `make release-check` (i.e. `goreleaser check`) clean — no deprecation warnings.
- [x] `make snapshot` produces:
  - `dist/dbsync_v*-next_linux_amd64.tar.gz` (berisi `dbsync` + `README.md` + `CHANGELOG.md`)
  - `dist/dbsync_v*-next_windows_amd64.zip` (berisi `dbsync.exe` + `README.md` + `CHANGELOG.md`)
  - `dist/dbsync_v*-next_checksums.txt`
- [x] `cd dist && sha256sum -c dbsync_v*-next_checksums.txt` → semua OK.
- [x] CI dry-run berhasil: push tag `v0.0.0-test` di branch throwaway → workflow jalan → GitHub Release published sebagai pre-release. Cleanup tag + release + branch setelahnya.

---

## ☑ Step 4 — S4 · `docs: add ADR-0002 + README installation/Windows/Releases + CONTEXT.md release section` ✅ DONE

**bd:** `dbsync-9ps`
**GH:** [`#16`](https://github.com/kentoespdam/dbsync/issues/16)
**Type:** AFK
**Blocked by:** S3
**Blocks:** S6

### What

Buat `docs/adr/0002-cross-platform-binary-distribution.md` (style ADR-0001). Update README dengan 3 section baru. Update CONTEXT.md (Stack append + new section + Pointer row + footer date). Lihat plan §"README.md outline" + §"ADR-0002 outline" + §"CONTEXT.md updates".

### Acceptance

- [x] `docs/adr/0002-*.md` ada, follow style ADR-0001 (Status: Accepted, Date 2026-05-28, sections Context → Decision → Alternatives → Trade-offs → Consequences → References).
- [x] README punya `## Installation`, `## Windows usage`, `## Releases` (di GitHub render correctly).
- [x] CONTEXT.md §Stack punya line "Release: GoReleaser v2 + GA…".
- [x] CONTEXT.md punya new section `## Release artifact` setelah "Logging dua-jalur".
- [x] CONTEXT.md Pointer table punya row "Distribusi & rilis".
- [x] CONTEXT.md footer di-bump ke `*Last updated: 2026-05-28.*`.

---

## ☑ Step 5 — S5 · `decision: choose project LICENSE before first public release` ✅ DONE

**bd:** `dbsync-kuj`
**GH:** [`#15`](https://github.com/kentoespdam/dbsync/issues/15)
**Type:** HITL (butuh keputusan dari project owner)
**Blocked by:** —
**Blocks:** S6

### What

User/maintainer pilih license (rekomendasi: MIT atau Apache-2.0). Tambah `LICENSE` di repo root. GoReleaser `.goreleaser.yml` sudah glob `LICENSE*` — otomatis ikut di archive setelah file ada.

### Acceptance

- [x] `LICENSE` ada di repo root.
- [x] `make snapshot` setelah merge S5 → archive berisi `LICENSE`.
- [x] ADR-0002 (kalau sudah merged dari S4) update reference jika perlu.

---

## ☑ Step 6 — S6 · `release: cut v1.0.0 + smoke-test Windows binary manually` 🧑‍🔬 HITL

**bd:** `dbsync-1kc`
**GH:** [`#14`](https://github.com/kentoespdam/dbsync/issues/14)
**Type:** HITL (butuh maintainer di mesin Windows real)
**Blocked by:** S3, S4, S5

### What

Tag `v1.0.0` di main, push, tunggu CI selesai, download Windows ZIP, extract ke `C:\dbsync\`, jalankan `dbsync.exe`, verifikasi:

1. SmartScreen warning muncul → click "More info" → "Run anyway" (UX yang di-document di README).
2. TUI render correctly (bubbletea + lipgloss).
3. Logs land di `C:\dbsync\logs\dbsync.log` (validasi ADR-0001 path logic di Windows).
4. SQLite `C:\dbsync\dbsync.db` ke-create.
5. SHA-256 checksum match.

### Acceptance

- [ ] Tag `v1.0.0` pushed ke origin.
- [ ] GitHub Release `v1.0.0` published dengan: linux tar.gz, windows zip, checksums.txt, auto-changelog body.
- [ ] Maintainer screenshot/log konfirmasi Windows smoke test (5 poin di atas) di-attach ke issue.
- [ ] No follow-up bug di-file dari smoke test (kalau ada, file `dbsync-XXX` bug terpisah).

---

## Follow-ups (file & defer, tidak block S6)

### F1 — `chore: rename module path github.com/user/dbsync → github.com/kentoespdam/dbsync`

**Type:** AFK · **Blocks:** — · `bd` issue: [`dbsync-da2`](https://github.com/kentoespdam/dbsync/issues/22). Kosmetik tapi best practice. `go mod edit -module` + sed semua import + verify build.

### F2 — `test: add MySQL integration test workflow (non-gating)`

**Type:** AFK · **Blocked by:** S3 · `bd` issue: [`dbsync-55p`](https://github.com/kentoespdam/dbsync/issues/21). Closes ADR-0002 trade-off #2. Spin up MySQL service container di GA workflow terpisah dari release; trigger di push main / PR. Tidak gate release.

### F3 — `release: add windows-latest smoke-test job for PRs touching internal/tui/`

**Type:** AFK · **Blocked by:** S3 · `bd` issue: [`dbsync-1ov`](https://github.com/kentoespdam/dbsync/issues/20). Closes ADR-0002 trade-off #3. Conditional GA job (paths filter) untuk catch bubbletea Windows quirks.

### F4 — `release: extend goreleaser matrix with linux/arm64 + darwin/{amd64,arm64} when demand arrives`

**Type:** AFK (speculative) · **Blocked by:** S3 · `bd` issue: [`dbsync-5tw`](https://github.com/kentoespdam/dbsync/issues/24). ADR-0002 alternative #2-#3. Trivial — 2 line di `.goreleaser.yml`. File now, defer execution.

### F5 — `release: evaluate Authenticode signing if Windows SmartScreen complaints accumulate` 🧑‍⚖️ HITL

**Type:** HITL · **Blocked by:** S3 · `bd` issue: [`dbsync-1wm`](https://github.com/kentoespdam/dbsync/issues/23). ADR-0002 trade-off #1. Track complaint count; revisit kalau >5 user-facing complaint.

---

## Quick reference

```bash
# Find next ready slice
bd ready

# Claim (ganti ID sesuai slice, mis. dbsync-28b untuk S1)
bd update <bd-id> --claim

# Local verify (mulai S3 onward)
make release-check
make snapshot
cd dist && sha256sum -c *_checksums.txt

# Close
bd close <bd-id>

# Session close (MANDATORY per CLAUDE.md)
git pull --rebase && bd dolt push && git push && git status
```
