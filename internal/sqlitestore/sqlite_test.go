package sqlitestore_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/ossewawiel/gowallet/internal/sqlitestore"
)

func TestOpen_SetsPRAGMAs(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "pragmas.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	db := store.DB()

	var journal string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatalf("read journal_mode: %v", err)
	}
	if journal != "wal" {
		t.Errorf("journal_mode: want wal, got %q", journal)
	}

	var foreignKeys int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&foreignKeys); err != nil {
		t.Fatalf("read foreign_keys: %v", err)
	}
	if foreignKeys != 1 {
		t.Errorf("foreign_keys: want 1, got %d", foreignKeys)
	}

	var busyTimeout int
	if err := db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout); err != nil {
		t.Fatalf("read busy_timeout: %v", err)
	}
	if busyTimeout != 5000 {
		t.Errorf("busy_timeout: want 5000, got %d", busyTimeout)
	}

	var synchronous int
	if err := db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatalf("read synchronous: %v", err)
	}
	if synchronous != 1 { // NORMAL == 1
		t.Errorf("synchronous: want 1 (NORMAL), got %d", synchronous)
	}
}

func TestMigrate_AppliesOnStartup(t *testing.T) {
	t.Parallel()

	dbPath := filepath.Join(t.TempDir(), "migrate.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	var name string
	err = store.DB().QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='goose_db_version'`,
	).Scan(&name)
	if err != nil {
		t.Fatalf("goose_db_version lookup: %v", err)
	}
	if name != "goose_db_version" {
		t.Fatalf("want goose_db_version table to exist, got %q", name)
	}
}
