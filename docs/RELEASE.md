# Release Guide

Tutorial rilis versi baru dbsync ke GitHub Releases.

Pipeline rilis = **tag-triggered**: push tag `v*` → workflow `.github/workflows/release.yml` jalan → GoReleaser build + upload artifact + bikin release notes.

## TL;DR

```bash
# 1. Pastikan main bersih & sync
git checkout main
git pull --rebase
git status      # harus clean

# 2. Update CHANGELOG.md (lihat bagian "Changelog" di bawah)
$EDITOR CHANGELOG.md
git add CHANGELOG.md
git commit -m "docs: changelog for vX.Y.Z"
git push

# 3. Tag + push
git tag -a vX.Y.Z -m "Release vX.Y.Z"
git push origin vX.Y.Z

# 4. Pantau workflow
gh run watch
gh release view vX.Y.Z
```

## Pilih nomor versi (SemVer)

Lihat commit sejak tag terakhir:

```bash
git log $(git describe --tags --abbrev=0)..HEAD --oneline
```

Aturan bump:

| Jenis perubahan                              | Bump  | Contoh           |
|----------------------------------------------|-------|------------------|
| Breaking change (CLI flag hilang, format DB) | major | v1.x → v2.0.0    |
| `feat:` baru, backward-compatible            | minor | v1.0 → v1.1.0    |
| `fix:`, `refactor:`, `chore:` saja           | patch | v1.1.0 → v1.1.1  |
| Pre-release / belum stabil                   | suffix| v1.2.0-rc.1      |

GoReleaser auto-deteksi `-rc`/`-alpha`/`-beta` sebagai prerelease (`prerelease: auto`).

## Changelog

Project ini punya **dua** sumber changelog yang harus dipikirkan:

### 1. `CHANGELOG.md` (manual)

Wajib di-update sebelum tag. File ini ikut di-bundle ke `.tar.gz`/`.zip` archive
(lihat `archives.files` di `.goreleaser.yml`).

Format `Keep a Changelog`:

```markdown
## [vX.Y.Z] - YYYY-MM-DD

### Added
- Fitur baru.

### Changed
- Perubahan behavior. Sebut ADR kalau relevan.

### Fixed
- Bug yang diperbaiki.

### Removed
- Fitur yang dihapus.

### CI
- Perubahan workflow / build (opsional, bukan keep-a-changelog standard).
```

Ambil ringkasan dari `git log <last-tag>..HEAD --oneline`. Skip commit `docs:` /
`chore:` kecuali ada konsekuensi untuk user.

### 2. Release notes di GitHub (auto)

GoReleaser generate dari git log dengan filter di `.goreleaser.yml`:

```yaml
changelog:
  use: git
  sort: asc
  filters:
    exclude: [^docs:, ^doc:, ^chore:, ^test:, ^ci:, Merge ...]
```

Artinya commit harus pakai prefix conventional commit yang **bukan** di-exclude
supaya muncul di release notes. Tidak perlu intervensi manual — cukup pastikan
commit message rapi sebelum tag.

## Pre-flight checklist

Sebelum tag:

- [ ] `main` up-to-date dengan `origin/main`, working tree bersih
- [ ] `go test ./...` hijau lokal (workflow juga akan jalanin, tapi cek dulu)
- [ ] `go build -o /tmp/dbsync ./cmd/dbsync` sukses
- [ ] `CHANGELOG.md` punya entry untuk versi baru dengan tanggal hari ini
- [ ] Nomor versi konsisten di mana-mana (CHANGELOG, dokumentasi rilis)
- [ ] Tidak ada secret / file lokal nyangkut (`git status` clean)

## Eksekusi rilis

```bash
git tag -a v1.1.0 -m "Release v1.1.0"
git push origin v1.1.0
```

Pakai **annotated tag** (`-a`), bukan lightweight. GoReleaser pakai metadata tag.

Setelah `git push origin v1.1.0`:

1. Workflow `Release` (`.github/workflows/release.yml`) jalan otomatis
2. Step `Unit tests` — `go test ./...`
3. Step `Run GoReleaser` — build linux+windows amd64, bikin archive + checksum
4. Upload ke `https://github.com/kentoespdam/dbsync/releases/tag/v1.1.0`

## Pantau & verifikasi

```bash
gh run list --workflow=release.yml --limit 3
gh run watch                              # tunggu selesai
gh release view v1.1.0                    # cek release notes & assets
gh release view v1.1.0 --web              # buka di browser
```

Asset yang diharapkan ada:

- `dbsync_v1.1.0_linux_amd64.tar.gz`
- `dbsync_v1.1.0_windows_amd64.zip`
- `dbsync_v1.1.0_checksums.txt`

## Rollback / fix kalau salah tag

### Tag belum di-push

```bash
git tag -d v1.1.0
```

### Tag sudah di-push tapi workflow belum selesai

```bash
git tag -d v1.1.0
git push --delete origin v1.1.0
# (opsional) batalkan workflow run
gh run cancel <run-id>
```

### Release sudah publish dan ada masalah

Jangan hapus release — bikin patch baru:

```bash
# perbaiki bug, commit
git tag -a v1.1.1 -m "Release v1.1.1"
git push origin v1.1.1
```

Hapus release lama via `gh release delete v1.1.0 --yes` **hanya** kalau release
broken parah (binary tidak bisa jalan). Tag tetap di repo sebagai history.

## Test rilis tanpa publish

Untuk verifikasi config GoReleaser lokal sebelum push tag:

```bash
goreleaser release --snapshot --clean --skip=publish
ls dist/                # cek artifact
```

Snapshot pakai versi `<next-patch>-next` sesuai `snapshot.version_template`.

## Referensi config

- `.goreleaser.yml` — build matrix, archive, changelog filter, release notes template
- `.github/workflows/release.yml` — trigger & runner setup
- `Makefile` — target `build`/`build-all` untuk dev build lokal dengan version injection
- ADR di `docs/adr/` — catat keputusan yang mempengaruhi user-facing behavior antar rilis
