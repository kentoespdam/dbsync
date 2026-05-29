# Changelog

All notable changes to this project will be documented in this file.

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
