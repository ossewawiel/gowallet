// Package sqlitestore owns everything SQLite: opening the database with the
// right PRAGMAs, running goose migrations, and implementing the repository
// interfaces defined in the wallet core (currently just wallet.Pinger).
//
// It is never imported by internal/httpapi — only cmd/gowallet wires it in.
package sqlitestore

import (
	"context"
	"database/sql"
	"embed"
	"fmt"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // pure-Go SQLite driver, registered as "sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// pragmas are applied to every connection via the DSN so WAL, the busy
// timeout, NORMAL durability, and FK enforcement hold on the single writer.
// Non-negotiable per docs/ARCHITECTURE.md.
const pragmas = "?_pragma=journal_mode(WAL)" +
	"&_pragma=busy_timeout(5000)" +
	"&_pragma=synchronous(NORMAL)" +
	"&_pragma=foreign_keys(ON)"

// Store is the SQLite-backed persistence handle. It owns one *sql.DB pool and
// satisfies wallet.Pinger.
type Store struct {
	db *sql.DB
}

// Open opens (creating if needed) the SQLite database at path, applies the
// required PRAGMAs, and pins the write path to a single connection so we never
// trip SQLITE_BUSY.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+pragmas)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Single writer — SQLite serialises writes; one conn avoids lock churn.
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	return &Store{db: db}, nil
}

// DB exposes the underlying pool. Used by sqlc-generated queries (later slices)
// and by tests that assert PRAGMAs / schema.
func (s *Store) DB() *sql.DB { return s.db }

// Close releases the pool.
func (s *Store) Close() error { return s.db.Close() }

// Ping satisfies wallet.Pinger — confirms the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Migrate applies all embedded goose migrations on startup. goose creates the
// goose_db_version bookkeeping table as a side effect.
func (s *Store) Migrate(ctx context.Context) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("goose dialect: %w", err)
	}
	if err := goose.UpContext(ctx, s.db, "migrations"); err != nil {
		return fmt.Errorf("goose up: %w", err)
	}
	return nil
}
