# Changelog

All notable changes to this project will be documented in this file.

## [v1.0.0] - 2026-05-29

### Changed
- Removed MySQL error redaction from app log and per-sync JSONL log. Errors are now written verbatim for actionability. Connection password redaction in `mysql/pool.go` is unchanged. See ADR 0003.

## [v1.0.0] - 2026-05-28

### Added
- Initial release with TUI and CLI support.
- Cross-platform support for Linux and Windows.
- SQLite backend for configuration and history.
- MySQL connectivity and schema inspection.
- One-way table synchronization.
- Encrypted credential storage.
