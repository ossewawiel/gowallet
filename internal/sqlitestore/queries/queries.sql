-- name: CreateAccount :exec
-- extended: now also stores the optional password_hash. role is deliberately
-- NOT in the column list, so it always takes the table default 'member'.
INSERT INTO accounts (account_id, name, password_hash) VALUES (?, ?, ?);

-- name: GetAccount :one
SELECT account_id, name, created_at FROM accounts WHERE account_id = ?;

-- name: GetAccountCredential :one
-- password_hash is nullable, so sqlc returns sql.NullString. A NULL (or absent
-- account) is treated by the service as an invalid credential (no enumeration).
SELECT password_hash, role FROM accounts WHERE account_id = ?;

-- name: AccountExists :one
SELECT EXISTS(SELECT 1 FROM accounts WHERE account_id = ?) AS present;

-- name: InsertTransaction :execresult
INSERT INTO transactions (ref, account_id, kind, points, occurred_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(ref) DO NOTHING;

-- name: GetTransactionByRef :one
SELECT ref, account_id, kind, points, occurred_at
FROM transactions
WHERE ref = ?;

-- name: BalanceForAccount :one
SELECT CAST(COALESCE(SUM(CASE WHEN kind = 'earn' THEN points ELSE -points END), 0) AS INTEGER) AS balance
FROM transactions
WHERE account_id = ?;

-- name: ListAccountsWithBalance :many
-- Each account joined to its derived balance (sum(earn) - sum(spend)) via a
-- correlated subquery -- the SAME formula as BalanceForAccount, so the list
-- balance can never drift from GET /balance. COALESCE(...,0) so an account with
-- no txns shows 0.
SELECT a.account_id, a.name, a.role,
       CAST(COALESCE((SELECT SUM(CASE WHEN t.kind = 'earn' THEN t.points ELSE -t.points END)
                      FROM transactions t WHERE t.account_id = a.account_id), 0) AS INTEGER) AS balance
FROM accounts a
ORDER BY a.account_id;

-- name: ListTransactionsByAccount :many
-- Newest-first by id (strictly monotonic AUTOINCREMENT) -- stable even when two
-- rows share an occurred_at. Account existence is checked by the caller first.
SELECT ref, kind, points, occurred_at
FROM transactions
WHERE account_id = ?
ORDER BY id DESC;

-- name: AppendAuditEntry :one
INSERT INTO audit_log (ref, account_id, kind, points, outcome, reason)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING id, ref, account_id, kind, points, outcome, reason, created_at;

-- name: ListAuditLog :many
SELECT id, ref, account_id, kind, points, outcome, reason, created_at
FROM audit_log
ORDER BY id DESC;

-- name: ListAuditLogByAccount :many
SELECT id, ref, account_id, kind, points, outcome, reason, created_at
FROM audit_log
WHERE account_id = ?
ORDER BY id DESC;
