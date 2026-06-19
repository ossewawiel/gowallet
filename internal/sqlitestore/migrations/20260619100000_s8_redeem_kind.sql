-- +goose Up
-- Rebuild transactions to (a) widen the kind CHECK to include 'redeem' and
-- (b) add a nullable reward column (set only on redeem rows). SQLite can't ALTER
-- a CHECK, so: create new → copy → drop → rename → recreate index. Preserves
-- UNIQUE(ref), the FK to accounts, the points>0 CHECK, AUTOINCREMENT ids, and
-- idx_transactions_account. FK-safe inside the txn: nothing references transactions.
CREATE TABLE transactions_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL UNIQUE,
    account_id  TEXT    NOT NULL REFERENCES accounts(account_id),
    kind        TEXT    NOT NULL CHECK (kind IN ('earn','spend','redeem')),
    points      INTEGER NOT NULL CHECK (points > 0),
    reward      TEXT,                                  -- nullable: set only on redeem rows
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

INSERT INTO transactions_new (id, ref, account_id, kind, points, reward, occurred_at, created_at)
SELECT id, ref, account_id, kind, points, NULL, occurred_at, created_at FROM transactions;

DROP TABLE transactions;
ALTER TABLE transactions_new RENAME TO transactions;

CREATE INDEX idx_transactions_account ON transactions(account_id);

-- +goose Down
-- Reverse the rebuild: narrow the CHECK back to ('earn','spend') and drop reward.
-- Lossy by design — any redeem rows are dropped (the feature is being rolled back).
CREATE TABLE transactions_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    ref         TEXT    NOT NULL UNIQUE,
    account_id  TEXT    NOT NULL REFERENCES accounts(account_id),
    kind        TEXT    NOT NULL CHECK (kind IN ('earn','spend')),
    points      INTEGER NOT NULL CHECK (points > 0),
    occurred_at TEXT    NOT NULL,
    created_at  TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%SZ','now'))
);

INSERT INTO transactions_old (id, ref, account_id, kind, points, occurred_at, created_at)
SELECT id, ref, account_id, kind, points, occurred_at, created_at
FROM transactions WHERE kind IN ('earn','spend');

DROP TABLE transactions;
ALTER TABLE transactions_old RENAME TO transactions;

CREATE INDEX idx_transactions_account ON transactions(account_id);
