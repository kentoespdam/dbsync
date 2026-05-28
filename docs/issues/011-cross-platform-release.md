# Issue 011 — Cross-platform binary distribution (Linux + Windows)

**Type:** AFK (mostly) + 2 HITL (LICENSE decision + first-release smoke test)
**Triage label:** `needs-triage`, `ready-for-agent` (per slice)
**Blocked by:** —
**Blocks:** (akan di-link setelah slice individual dibuat: `dbsync-XXX` chain)
**Parent:** [`CONTEXT.md`](../../CONTEXT.md), [`docs/PRD-v1.md`](../PRD-v1.md), [`docs/ARCHITECTURE.md`](../ARCHITECTURE.md)
**Related ADR (akan ditulis di S4):** `docs/adr/0002-cross-platform-binary-distribution.md`

---

## ⚠️ Wajib dibaca dulu (untuk Agent yang ngerjain)

1. **`CLAUDE.md`** project root — DRY comment, max 120 baris/file, `context7` wajib untuk lib eksternal, `bd` untuk task tracking (BUKAN TodoWrite/TaskCreate), session close protocol (`git push` + `bd dolt push`).
2. **`CONTEXT.md`** root — §Stack, §Logging dua-jalur, §Pointer table (akan ditambah row "Distribusi & rilis" di S4).
3. **`docs/adr/0001-application-logging-strategy.md`** — paths binary-relative (`<exeDir>/logs/...`) berlaku sama di Linux dan Windows. Jangan asumsikan `/opt/` atau `~/` style.
4. **`docs/issues/011-claim-order.md`** — claim order linear, satu agent satu slice, jangan paralel.
5. **GoReleaser v2 docs** (`context7` query untuk `goreleaser`) — syntax sudah berubah dari v1 (`format:` → `formats:` plural). Jangan pakai pengetahuan v1.

---

## Latar belakang (ringkas)

`dbsync` selesai v1 (TUI + CLI satu binary). Distribusi saat ini = `make build` di mesin dev → manual `scp` ke server. Tidak skala untuk:

1. **Operator Windows** yang mau pakai dbsync di workstation untuk sync ad-hoc.
2. **Audit trail rilis** — tidak ada artefak ter-versioned di GitHub Release, susah trace "versi mana yang jalan di server X".
3. **Reproducibility** — `go build` lokal tergantung Go version + GOPATH + env dev.

**Codebase sudah portable by accident:**
- Pure Go (`modernc.org/sqlite`, `go-sql-driver/mysql`, bubbletea, `golang.org/x/term`).
- Tidak ada `runtime.GOOS` check, tidak ada build tag platform.
- Semua path binary-relative via `os.Executable()` (`internal/paths/paths.go`, `internal/config/config.go`).
- `cmd/dbsync/main.go:27` — `var version = "v1.0.0-dev"` sudah siap di-override via `-ldflags`.

Gap **hanya di build/release infrastructure**, bukan kode aplikasi.

---

## Confirmed scope (hasil grilling 2026-05-28)

| Keputusan | Pilihan |
|---|---|
| Scope | Local cross-compile + automated GitHub Release on git tag push |
| Targets | `linux/amd64` + `windows/amd64` (no arm64, no darwin) |
| Artifact format | `.tar.gz` (Linux) + `.zip` (Windows), berisi `dbsync(.exe)` + `README.md` + auto-generated `CHANGELOG.md` |
| Naming | `dbsync_v{version}_{os}_{arch}.{ext}` — lowercase |
| Versioning | SemVer dari git tag, inject via `-ldflags -X main.version=...` |
| Tooling | GoReleaser v2 (single source of truth); Makefile delegates |
| CI trigger | Tag push `v*` only; runner: `ubuntu-latest` only (pure-Go cross-compile) |
| Build flags | `-trimpath -ldflags="-s -w -X main.version=..."` |
| Integrity | SHA-256 `checksums.txt`; **tidak ada code signing** (no Sigstore/GPG/Authenticode). Windows SmartScreen warning di-accept. |
| Test gate | `go test ./...` (unit) wajib pass sebelum goreleaser. Integration test (`-tags=integration`) **tidak** di CI. |
| Docs | Update README (Installation + Windows usage + Releases) + add ADR-0002 + CONTEXT.md |
| LICENSE | **Open question** — repo belum ada LICENSE. Diputuskan di S5 (HITL). |

---

## File yang dibuat / diubah

| Action | Path | Slice |
|---|---|---|
| CREATE | `.gitignore` (append `dist/`) | S1 |
| MODIFY | `Makefile` (vars + `build-linux`/`build-windows`/`build-all`/`snapshot`/`release-check`, update `build` & `clean`) | S2 |
| CREATE | `.goreleaser.yml` | S3 |
| CREATE | `.github/workflows/release.yml` | S3 |
| MODIFY | `README.md` (sections: Installation + Windows usage + Releases) | S4 |
| CREATE | `docs/adr/0002-cross-platform-binary-distribution.md` | S4 |
| MODIFY | `CONTEXT.md` (Stack append + new §"Release artifact" + Pointer row + footer date) | S4 |
| CREATE | `LICENSE` | S5 (HITL) |
| NO CHANGE | `cmd/dbsync/main.go` | — (`var version` di line 27 sudah siap, fallback `v1.0.0-dev` intentional) |

---

## Konten file kunci

### `.goreleaser.yml` (full)

> Validated terhadap schema GoReleaser v2 (2026-05). `formats:` (plural). `version: 2` wajib.

```yaml
version: 2

project_name: dbsync

before:
  hooks:
    - go mod tidy
    - go vet ./...

builds:
  - id: dbsync
    main: ./cmd/dbsync
    binary: dbsync
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
    goarch:
      - amd64
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version=v{{ .Version }}
    mod_timestamp: "{{ .CommitTimestamp }}"

archives:
  - id: default
    ids:
      - dbsync
    name_template: "{{ .ProjectName }}_v{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    formats:
      - tar.gz
    format_overrides:
      - goos: windows
        formats:
          - zip
    files:
      - README.md
      - CHANGELOG.md
      - LICENSE*
    wrap_in_directory: false

checksum:
  name_template: "{{ .ProjectName }}_v{{ .Version }}_checksums.txt"
  algorithm: sha256

snapshot:
  version_template: "{{ incpatch .Version }}-next"

changelog:
  use: git
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^doc:"
      - "^chore:"
      - "^test:"
      - "^ci:"
      - "Merge pull request"
      - "Merge branch"

release:
  github:
    owner: kentoespdam
    name: dbsync
  draft: false
  prerelease: auto
  name_template: "dbsync v{{ .Version }}"
  header: |
    ## dbsync v{{ .Version }}

    Single-binary MySQL one-way table sync (TUI + CLI), cron-friendly.
  footer: |
    ---
    SHA-256 checksums in `{{ .ProjectName }}_v{{ .Version }}_checksums.txt`.
    No code signing — Windows users may see SmartScreen warning on first run; see README.
```

### `.github/workflows/release.yml` (full)

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    name: GoReleaser
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0  # required for changelog

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - name: Unit tests
        run: go test ./...

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

### Makefile additions

Variabel baru di atas:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath
```

Extend `.PHONY`:

```makefile
.PHONY: all build run test test-integration clean fmt vet help \
        build-linux build-windows build-all snapshot release-check
```

Update `build` (konsisten dengan release):

```makefile
build:
	@echo "Building $(BINARY_NAME) $(VERSION)..."
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_FILE)
```

Target baru:

```makefile
build-linux:
	@echo "Building $(BINARY_NAME) for linux/amd64 ($(VERSION))..."
	@mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY_NAME)_linux_amd64 $(MAIN_FILE)

build-windows:
	@echo "Building $(BINARY_NAME) for windows/amd64 ($(VERSION))..."
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
		go build $(GOFLAGS) -ldflags "$(LDFLAGS)" \
		-o dist/$(BINARY_NAME)_windows_amd64.exe $(MAIN_FILE)

build-all: build-linux build-windows
	@echo "Cross-platform builds done. See ./dist/"

snapshot:
	@echo "Running GoReleaser snapshot..."
	goreleaser release --snapshot --clean

release-check:
	@goreleaser check
```

Update `clean`:

```makefile
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY_NAME)
	rm -rf dist/
```

Tambah baris ke `help` block (4 line baru).

### `.gitignore` append

```
dist/
```

### README.md outline (S4)

Tambah section setelah usage:

- `## Installation` — Download pre-built binary (link releases page) + verify SHA-256 + Build from source.
- `## Windows usage` — Install (`C:\dbsync\`), SmartScreen warning (klik "More info" → "Run anyway"), Paths on Windows (logs `C:\dbsync\logs\`, SQLite `C:\dbsync\dbsync.db`, salt `%USERPROFILE%\.config\dbsync\salt`), Scheduling dengan Task Scheduler (action: `C:\dbsync\dbsync.exe`; args: `run --connection=prod --table=invoices`; trigger daily; env var `DBSYNC_MASTER_KEY`).
- `## Releases` — SemVer tag → CI → GoReleaser → GitHub Release; cut release: `git tag vX.Y.Z && git push --tags`; local dry-run: `make snapshot`.

### ADR-0002 outline (S4)

Match style ADR-0001 (Bahasa + English technical terms, Date 2026-05-28). Sections:

- **Context** — distribusi manual tidak skala; codebase sudah portable by accident.
- **Decision** — GoReleaser v2 + GA on tag. Matrix linux/amd64 + windows/amd64. Naming, versioning, build flags, no signing, test gate, local dev (snapshot vs build-all).
- **Alternatives considered** (semua dengan reject reason): per-PR matrix build, darwin, linux/arm64, package managers, Sigstore/cosign, Authenticode, Windows runner, manual `gh release upload`.
- **Trade-offs / Accepted risks** — SmartScreen UX, no integration test, no Windows smoke test in CI, ldflags symbol path dependency, GoReleaser sebagai new dep, `dist/` cleanup.
- **Consequences** — tag push = release, audit trail, operator Windows tanpa Go, reproducible; cost: 2 file baru, maintainer wajib paham flow.
- **References** — CONTEXT.md, Makefile, `.goreleaser.yml`, workflow, `cmd/dbsync/main.go:27`, GoReleaser v2 docs, ADR-0001.

### CONTEXT.md updates (S4)

1. Append ke §Stack: `**Release:** GoReleaser v2 + GitHub Actions on tag push. Targets: linux/amd64 + windows/amd64.`
2. Section baru setelah §"Logging dua-jalur":

   ```markdown
   ## Release artifact

   - Tag SemVer `vX.Y.Z` di `main` → GitHub Actions (`.github/workflows/release.yml`) → GoReleaser → publish ke GitHub Release.
   - Artefak: `dbsync_v{version}_linux_amd64.tar.gz`, `dbsync_v{version}_windows_amd64.zip`, `dbsync_v{version}_checksums.txt` (SHA-256).
   - Version di-inject via `-ldflags "-X main.version=vX.Y.Z"` ke `cmd/dbsync/main.go:27`. Fallback dev: `v1.0.0-dev`.
   - Local dry-run: `make snapshot` (butuh `goreleaser` CLI). Quick cross-build: `make build-all`.
   - Lihat ADR-0002.
   ```

3. Tambah row di Pointer table: `| Distribusi & rilis | docs/adr/0002-cross-platform-binary-distribution.md |`
4. Bump footer ke `*Last updated: 2026-05-28.*`.

---

## Verifikasi end-to-end

### Per slice

Lihat acceptance criteria di tiap bd issue. Singkatnya:

- **S1 done:** `git status` setelah `make build-all` tidak menampilkan `dist/` sebagai untracked.
- **S2 done:** `make build-all` produces `dist/dbsync_linux_amd64` + `dist/dbsync_windows_amd64.exe`; `./dist/dbsync_linux_amd64 --version` print tag-derived version (BUKAN `v1.0.0-dev`).
- **S3 done:** `make release-check` clean; `make snapshot` produces archives + checksum di `dist/`; CI dry-run dengan tag `v0.0.0-test` publish ke GitHub Release (pre-release).
- **S4 done:** ADR-0002 ada, README punya 3 section baru, CONTEXT.md pointer table punya row baru, rendering di GitHub OK.
- **S5 done:** `LICENSE` ada di repo root; GoReleaser archive contains it.
- **S6 done:** `v1.0.0` tag pushed; GitHub Release published dengan 2 archive + checksum; maintainer manual smoke test `dbsync.exe` di mesin Windows real (SmartScreen → TUI render → log file).

### Pre-merge checklist tiap slice

- [ ] `var version = "v1.0.0-dev"` di `cmd/dbsync/main.go:27` **tidak diubah** (intentional dev fallback).
- [ ] `make release-check` pass (mulai dari S3 onward).
- [ ] `dist/` di `.gitignore` (verifikasi dari S1).
- [ ] `bd close <id>` setelah PR merged.

---

## Open items / risks (sudah jadi follow-up bd issue)

| Item | Severity | Slice |
|---|---|---|
| LICENSE absence | High | **S5** (blocks S6) |
| Module path `github.com/user/dbsync` vs `kentoespdam/dbsync` mismatch | Low | **F1** |
| No integration test in release pipeline | Medium | **F2** |
| No Windows smoke test in CI | Medium | **F3** |
| arm64 / darwin demand may emerge | Low | **F4** |
| Windows SmartScreen UX friction | Medium | **F5** |

---

## Referensi

- Plan source (private): `/home/dev/.claude/plans/aku-ingin-aplikasi-ini-fancy-kite.md`
- ADR-0001 (logging) — referensi style untuk ADR-0002 + landasan path binary-relative
- [GoReleaser v2 docs](https://goreleaser.com/customization/)
- [GoReleaser Action v6](https://github.com/goreleaser/goreleaser-action)
- [actions/setup-go v5](https://github.com/actions/setup-go)
