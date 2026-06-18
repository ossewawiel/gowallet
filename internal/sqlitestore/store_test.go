package sqlitestore_test

import (
	"context"
	"errors"
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

func spendTxn(ref, id string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindSpend,
		Points:     pts,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

// seedEarn records an earn and fails the test if the store rejects it.
func seedEarn(t *testing.T, store *sqlitestore.Store, ref, id string, pts int64) {
	t.Helper()
	if _, _, err := store.RecordTransaction(context.Background(), earnTxn(ref, id, pts)); err != nil {
		t.Fatalf("seed earn: %v", err)
	}
}

// INV-3 at the store seam: a spend that would go negative is rejected AND the
// post-insert-then-rollback guard leaves no row behind (balance unchanged).
func TestRecordSpend_BelowZero_Rejected_NoWrite(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	seedEarn(t, store, "earn-1", "member-1", 100)

	_, _, err := store.RecordTransaction(ctx, spendTxn("spend-1", "member-1", 150))
	if !errors.Is(err, wallet.ErrInsufficientBalance) {
		t.Fatalf("spend over balance: want ErrInsufficientBalance, got %v", err)
	}

	// The rollback must have undone the insert — balance still 100, and the
	// rejected ref must be free to reuse later (no orphan row).
	if bal, err := store.Balance(ctx, "member-1"); err != nil || bal != 100 {
		t.Fatalf("balance after rejected spend: want 100 (err nil), got %d (err %v)", bal, err)
	}
}

// Boundary: spending exactly to zero is allowed (balance - points == 0, not < 0).
func TestRecordSpend_ExactToZero_Allowed(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	seedEarn(t, store, "earn-1", "member-1", 100)

	if _, created, err := store.RecordTransaction(ctx, spendTxn("spend-1", "member-1", 100)); err != nil || !created {
		t.Fatalf("spend exact to zero: want created=true err nil, got created=%v err %v", created, err)
	}
	if bal, err := store.Balance(ctx, "member-1"); err != nil || bal != 0 {
		t.Fatalf("balance after exact spend: want 0, got %d (err %v)", bal, err)
	}
}

// A replayed spend is debited once: the first write already passed the balance
// check; the second hits ON CONFLICT DO NOTHING → no second check, no double debit.
func TestRecordSpend_DuplicateRef_Replay(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	seedEarn(t, store, "earn-1", "member-1", 100)

	first, created, err := store.RecordTransaction(ctx, spendTxn("spend-dup", "member-1", 40))
	if err != nil || !created {
		t.Fatalf("first spend: want created=true err nil, got created=%v err %v", created, err)
	}
	second, created, err := store.RecordTransaction(ctx, spendTxn("spend-dup", "member-1", 40))
	if err != nil {
		t.Fatalf("replay spend: %v", err)
	}
	if created {
		t.Fatalf("replay spend: want created=false")
	}
	if second.Points != first.Points {
		t.Fatalf("replay points: want %d, got %d", first.Points, second.Points)
	}
	if bal, err := store.Balance(ctx, "member-1"); err != nil || bal != 60 {
		t.Fatalf("balance after duplicate spend: want 60 (debited once), got %d (err %v)", bal, err)
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
