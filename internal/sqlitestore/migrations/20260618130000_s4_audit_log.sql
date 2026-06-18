-- +goose Up
CREATE TABLE audit_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL,                  -- NOT unique: append-only; the same ref can be attempted many times
    account_id  TEXT    NOT NULL,                  -- NO foreign key: must record attempts against unknown accounts too
    kind        TEXT    NOT NULL,                  -- NO check: a rejected attempt may carry an invalid kind (faithful record)
    points      INTEGER NOT NULL,                  -- NO check: a rejected attempt may have 0/negative points
    outcome     TEXT    NOT NULL CHECK (outcome IN ('accepted','rejected','duplicate')),  -- OUR controlled vocabulary → constrain it
    reason      TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))           -- RFC 3339 UTC, second precision
);

CREATE INDEX idx_audit_account ON audit_log(account_id);  -- supports the ?account_id= filter

-- +goose Down
DROP TABLE audit_log;
