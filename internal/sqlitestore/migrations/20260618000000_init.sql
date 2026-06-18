-- +goose Up
-- Initial migration — no domain tables yet.
-- Purpose: prove the goose runner wires up and applies on startup
-- (goose creates goose_db_version as a side effect). S1 adds accounts + transactions.
SELECT 1;

-- +goose Down
-- Nothing to undo.
SELECT 1;
