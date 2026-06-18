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

// auditEntry builds an AuditEntry for the store tests.
func auditEntry(ref, accountID string, outcome wallet.AuditOutcome, reason string) wallet.AuditEntry {
	return wallet.AuditEntry{
		Ref:       ref,
		AccountID: accountID,
		Kind:      "earn",
		Points:    10,
		Outcome:   outcome,
		Reason:    reason,
	}
}

// TestAudit_RecordsEachAttempt (INV-11) — each attempt (accepted / rejected /
// duplicate) is recorded and reads back with its reason and a parseable,
// non-zero created_at.
func TestAudit_RecordsEachAttempt(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	cases := []struct {
		ref     string
		outcome wallet.AuditOutcome
		reason  string
	}{
		{"a-1", wallet.OutcomeAccepted, "ok"},
		{"a-2", wallet.OutcomeRejected, "account not found"},
		{"a-3", wallet.OutcomeDuplicate, "duplicate ref"},
	}
	for _, c := range cases {
		got, err := store.AppendAudit(ctx, auditEntry(c.ref, "member-1", c.outcome, c.reason))
		if err != nil {
			t.Fatalf("AppendAudit %s: %v", c.ref, err)
		}
		if got.ID == 0 {
			t.Fatalf("AppendAudit %s: want assigned id, got 0", c.ref)
		}
		if got.Reason != c.reason {
			t.Fatalf("AppendAudit %s: reason want %q, got %q", c.ref, c.reason, got.Reason)
		}
		if got.CreatedAt.IsZero() {
			t.Fatalf("AppendAudit %s: created_at is zero", c.ref)
		}
	}

	list, err := store.ListAudit(ctx, "")
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListAudit: want 3 rows, got %d", len(list))
	}
	for _, e := range list {
		if e.Reason == "" || e.CreatedAt.IsZero() {
			t.Fatalf("row %q: want non-empty reason + non-zero created_at, got reason=%q created_at=%v", e.Ref, e.Reason, e.CreatedAt)
		}
	}
}

// TestAudit_AppendOnly_SameRefTwice (INV-22) — the same ref recorded twice yields
// two distinct rows (no UNIQUE(ref) collision — the opposite of transactions).
func TestAudit_AppendOnly_SameRefTwice(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	first, err := store.AppendAudit(ctx, auditEntry("tx-1", "member-1", wallet.OutcomeAccepted, "first"))
	if err != nil {
		t.Fatalf("first AppendAudit: %v", err)
	}
	second, err := store.AppendAudit(ctx, auditEntry("tx-1", "member-1", wallet.OutcomeDuplicate, "second"))
	if err != nil {
		t.Fatalf("second AppendAudit: %v", err)
	}
	if first.ID == second.ID {
		t.Fatalf("append-only: want distinct ids, both got %d", first.ID)
	}

	list, err := store.ListAudit(ctx, "")
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	var withRef int
	for _, e := range list {
		if e.Ref == "tx-1" {
			withRef++
		}
	}
	if withRef != 2 {
		t.Fatalf("append-only: want 2 rows with ref tx-1, got %d", withRef)
	}
}

// TestAudit_ListNewestFirst — rows come back ordered by id DESC (newest first).
func TestAudit_ListNewestFirst(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	refs := []string{"r-1", "r-2", "r-3"}
	for _, ref := range refs {
		if _, err := store.AppendAudit(ctx, auditEntry(ref, "member-1", wallet.OutcomeAccepted, "ok")); err != nil {
			t.Fatalf("AppendAudit %s: %v", ref, err)
		}
	}

	list, err := store.ListAudit(ctx, "")
	if err != nil {
		t.Fatalf("ListAudit: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("ListAudit: want 3, got %d", len(list))
	}
	// Newest-first → reverse insertion order, and strictly descending ids.
	want := []string{"r-3", "r-2", "r-1"}
	for i, e := range list {
		if e.Ref != want[i] {
			t.Fatalf("newest-first: position %d want %q, got %q", i, want[i], e.Ref)
		}
		if i > 0 && list[i-1].ID <= e.ID {
			t.Fatalf("newest-first: ids not strictly descending at %d (%d <= %d)", i, list[i-1].ID, e.ID)
		}
	}
}

// TestAudit_ListByAccount_FiltersAndNoLeak — filtering by account returns only
// that account's rows; no cross-account leak.
func TestAudit_ListByAccount_FiltersAndNoLeak(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()

	if _, err := store.AppendAudit(ctx, auditEntry("m1-a", "member-1", wallet.OutcomeAccepted, "ok")); err != nil {
		t.Fatalf("append m1-a: %v", err)
	}
	if _, err := store.AppendAudit(ctx, auditEntry("m2-a", "member-2", wallet.OutcomeAccepted, "ok")); err != nil {
		t.Fatalf("append m2-a: %v", err)
	}
	if _, err := store.AppendAudit(ctx, auditEntry("m1-b", "member-1", wallet.OutcomeRejected, "no")); err != nil {
		t.Fatalf("append m1-b: %v", err)
	}

	got, err := store.ListAudit(ctx, "member-1")
	if err != nil {
		t.Fatalf("ListAudit member-1: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("filter member-1: want 2 rows, got %d", len(got))
	}
	for _, e := range got {
		if e.AccountID != "member-1" {
			t.Fatalf("filter leak: got row for %q", e.AccountID)
		}
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
