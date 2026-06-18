package sqlitestore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/sqlitestore"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// openMigrated opens a fresh temp on-disk db and applies migrations.
func openMigrated(t *testing.T) *sqlitestore.Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "store.db")
	store, err := sqlitestore.Open(dbPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err := store.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return store
}

func earnTxn(ref, id string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindEarn,
		Points:     pts,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

func TestStore_InsertDuplicateRef_SecondIsNoOp(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	first, created, err := store.RecordTransaction(ctx, earnTxn("tx-1", "member-1", 150))
	if err != nil {
		t.Fatalf("first RecordTransaction: %v", err)
	}
	if !created {
		t.Fatalf("first insert: want created=true")
	}

	second, created, err := store.RecordTransaction(ctx, earnTxn("tx-1", "member-1", 999))
	if err != nil {
		t.Fatalf("second RecordTransaction: %v", err)
	}
	if created {
		t.Fatalf("second insert with same ref: want created=false (no-op)")
	}
	if second.Points != first.Points {
		t.Fatalf("replay points: want stored %d, got %d", first.Points, second.Points)
	}

	// Exactly one row physically stored → balance counts it once.
	bal, err := store.Balance(ctx, "member-1")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal != 150 {
		t.Fatalf("balance after duplicate: want 150, got %d", bal)
	}
}

func TestStore_Balance_DerivedFromRows(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}

	for i, pts := range []int64{100, 50, 25} {
		ref := "tx-" + string(rune('a'+i))
		if _, _, err := store.RecordTransaction(ctx, earnTxn(ref, "member-1", pts)); err != nil {
			t.Fatalf("RecordTransaction %d: %v", i, err)
		}
	}

	bal, err := store.Balance(ctx, "member-1")
	if err != nil {
		t.Fatalf("Balance: %v", err)
	}
	if bal != 175 {
		t.Fatalf("balance: want 175, got %d", bal)
	}
}
