-- +goose Up
-- password_hash is NULLABLE: an account with no secret simply can't log in.
ALTER TABLE accounts ADD COLUMN password_hash TEXT;
-- role defaults to 'member'; admin is seed-only (never settable through the API).
ALTER TABLE accounts ADD COLUMN role TEXT NOT NULL DEFAULT 'member'
    CHECK (role IN ('member','admin'));

-- Seed the two demo credentials. bcrypt (cost 12) embeds its own salt, so any
-- valid hash of the password verifies — the literals below are pre-generated.
-- ⚠️ TEMPORARY demo measure (published creds) — see README "Test credentials".
--   member-123 / demo-member-pw   (role member)
--   admin-001  / demo-admin-pw    (role admin)
INSERT INTO accounts (account_id, name, password_hash, role) VALUES
  ('member-123', 'Demo Member', '$2a$12$7gg9DxyphAr97IhIsioBFOkkZ6lyb3zdtevV/Q9lb9.CxD/R1kC/C', 'member'),
  ('admin-001',  'Demo Admin',  '$2a$12$LvEsOpgtZcSXg09X9vIgQeFp57IJxh35CAAcKhJJQExocBPl/Fta.',  'admin');

-- +goose Down
DELETE FROM accounts WHERE account_id IN ('member-123','admin-001');
ALTER TABLE accounts DROP COLUMN role;
ALTER TABLE accounts DROP COLUMN password_hash;
