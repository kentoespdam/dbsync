# Changelog

All notable changes to this project will be documented in this file.

## [v1.2.0] - 2026-05-29

### Added
- ValueMap editor in TUI mapping edit form (ENUM only) (bd-13e).
- Detect ENUM domain mismatch in AutoMap (bd-13d).
- CLI `--value-map` and `--value-map-file` flags for column value translation (bd-13c).
- Engine Resolve extended with ValueMap lookup and config validation (bd-13b).
- Storage support for ValueMap enum translation read/write (bd-13a).
- TUI list screen ENUM mismatch warning + save guard (bd-14b).
- ADR 0005: Value Map for enum translation.

### Fixed
- Value map form focus, browse button, and label display in edit form (bd-14a).

## [v1.1.0] - 2026-05-29

### Changed
- Removed MySQL error redaction from app log and per-sync JSONL log.
  Errors are now written verbatim for actionability. Connection password
  redaction in `mysql/pool.go` is unchanged. See ADR 0003.
- Renamed Go module path `github.com/user/dbsync` → `github.com/kentoespdam/dbsync`.

### Added
- Windows smoke-test workflow for TUI PRs.
- SmartScreen feedback issue template for F5 tracking.

### CI
- Added MySQL integration test workflow (non-gating).
- Dropped MySQL integration test container on GitHub free tier.

## [v1.0.0] - 2026-05-28

### Added
- Initial release with TUI and CLI support.
- Cross-platform support for Linux and Windows.
- SQLite backend for configuration and history.
- MySQL connectivity and schema inspection.
- One-way table synchronization.
- Encrypted credential storage.
