-- ============================================================
-- dbsync v1 — bd-13a: ValueMap storage
-- ============================================================

ALTER TABLE sync_column_mappings ADD COLUMN value_map TEXT
    CHECK (value_map IS NULL OR json_valid(value_map));
