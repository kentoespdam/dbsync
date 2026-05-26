-- ============================================================
-- dbsync v1 — initial schema
-- File: 001_init.sql
-- ============================================================
-- Run idempotently via `CREATE TABLE IF NOT EXISTS`.
-- Note: PRAGMA foreign_keys must also be enabled per-connection
-- from Go code (`PRAGMA foreign_keys = ON;` after Open).
-- ============================================================

-- ------------------------------------------------------------
-- connections: pair source+dest dalam 1 row (alias by name)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS connections (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT    NOT NULL UNIQUE,
    source_host     TEXT    NOT NULL,
    source_port     INTEGER NOT NULL DEFAULT 3306,
    source_user     TEXT    NOT NULL,
    source_password TEXT    NOT NULL,            -- base64(AES-256-GCM(nonce||ciphertext))
    source_db       TEXT    NOT NULL,
    dest_host       TEXT    NOT NULL,
    dest_port       INTEGER NOT NULL DEFAULT 3306,
    dest_user       TEXT    NOT NULL,
    dest_password   TEXT    NOT NULL,            -- base64(AES-256-GCM(nonce||ciphertext))
    dest_db         TEXT    NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- ------------------------------------------------------------
-- sync_column_mappings: mapping kolom per (connection, table)
-- ------------------------------------------------------------
-- Cases:
--   1) source_column NOT NULL, default_value NULL
--      → ambil nilai dari source.source_column → tulis ke dest.dest_column
--   2) source_column NULL, default_value NOT NULL
--      → tulis literal default_value ke dest.dest_column (mis. NOW())
--   3) source_column NOT NULL, default_value NOT NULL
--      → fallback: pakai source kalau NULL, kalau tetap NULL pakai default
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_column_mappings (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id   INTEGER NOT NULL,
    table_name      TEXT    NOT NULL,
    source_column   TEXT,
    dest_column     TEXT    NOT NULL,
    default_value   TEXT,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE,
    UNIQUE (connection_id, table_name, dest_column)
);

CREATE INDEX IF NOT EXISTS idx_mappings_conn_table
    ON sync_column_mappings(connection_id, table_name);

-- ------------------------------------------------------------
-- sync_checkpoints: posisi resume per (connection, table)
-- ------------------------------------------------------------
-- last_pk_value disimpan TEXT karena PK bisa INT/BIGINT/UUID/string.
-- Parsing tipe sebenarnya ditentukan runtime dari INFORMATION_SCHEMA.
-- UNIQUE per (connection, table): 1 checkpoint aktif per tabel;
-- riwayat lengkap ada di sync_history.
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_checkpoints (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id         INTEGER NOT NULL,
    table_name            TEXT    NOT NULL,
    last_batch_completed  INTEGER NOT NULL DEFAULT 0,
    last_pk_value         TEXT    NOT NULL DEFAULT '0',
    started_at            DATETIME NOT NULL,
    updated_at            DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    status                TEXT    NOT NULL CHECK (status IN ('running','interrupted','completed','failed')),
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE,
    UNIQUE (connection_id, table_name)
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_status
    ON sync_checkpoints(status);

-- ------------------------------------------------------------
-- sync_history: audit trail semua run (append-only)
-- ------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sync_history (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    connection_id     INTEGER NOT NULL,
    table_name        TEXT    NOT NULL,
    started_at        DATETIME NOT NULL,
    finished_at       DATETIME,
    duration_seconds  INTEGER,
    total_rows        INTEGER,
    success_rows      INTEGER,
    failed_rows       INTEGER,
    status            TEXT    NOT NULL CHECK (status IN ('running','completed','failed','interrupted')),
    error_summary     TEXT,
    FOREIGN KEY (connection_id) REFERENCES connections(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_history_conn_table_time
    ON sync_history(connection_id, table_name, started_at DESC);

-- ------------------------------------------------------------
-- Trigger: auto-bump updated_at di connections
-- ------------------------------------------------------------
CREATE TRIGGER IF NOT EXISTS trg_connections_updated_at
AFTER UPDATE ON connections
FOR EACH ROW
BEGIN
    UPDATE connections SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;

-- ------------------------------------------------------------
-- Trigger: auto-bump updated_at di checkpoints
-- ------------------------------------------------------------
CREATE TRIGGER IF NOT EXISTS trg_checkpoints_updated_at
AFTER UPDATE ON sync_checkpoints
FOR EACH ROW
BEGIN
    UPDATE sync_checkpoints SET updated_at = CURRENT_TIMESTAMP WHERE id = OLD.id;
END;
