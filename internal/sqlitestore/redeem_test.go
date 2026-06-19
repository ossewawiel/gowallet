package sqlitestore_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// redeemTxn builds a redeem-shaped Transaction (carries a reward) for the store tests.
func redeemTxn(ref, id, reward string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindRedeem,
		Points:     pts,
		Reward:     reward,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

// INV-25 at the store seam: a redeem that would go negative is rejected by the
// SAME in-tx guard as spend, and the post-insert rollback leaves no row behind.
func TestRecordTransaction_RedeemGuard_RollsBackBelowZero(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}, ""); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	seedEarn(t, store, "earn-1", "member-1", 100)

	_, _, err := store.RecordTransaction(ctx, redeemTxn("rdm-1", "member-1", "R150 voucher", 150))
	if !errors.Is(err, wallet.ErrInsufficientBalance) {
		t.Fatalf("redeem over balance: want ErrInsufficientBalance, got %v", err)
	}

	// Rollback must have undone the insert — balance still 100, ref free to reuse.
	if bal, err := store.Balance(ctx, "member-1"); err != nil || bal != 100 {
		t.Fatalf("balance after rejected redeem: want 100 (err nil), got %d (err %v)", bal, err)
	}
}

// A redeem row round-trips its reward through the store; earn/spend rows store
// NULL/empty reward.
func TestRecordTransaction_RedeemStoresReward(t *testing.T) {
	store := openMigrated(t)
	ctx := context.Background()
	if err := store.CreateAccount(ctx, wallet.Account{ID: "member-1", Name: "Rina"}, ""); err != nil {
		t.Fatalf("CreateAccount: %v", err)
	}
	seedEarn(t, store, "earn-1", "member-1", 300)

	stored, created, err := store.RecordTransaction(ctx, redeemTxn("rdm-1", "member-1", "R50 voucher", 50))
	if err != nil || !created {
		t.Fatalf("redeem: want created=true err nil, got created=%v err %v", created, err)
	}
	if stored.Reward != "R50 voucher" {
		t.Fatalf("stored reward: want %q, got %q", "R50 voucher", stored.Reward)
	}
	if stored.Kind != wallet.KindRedeem {
		t.Fatalf("stored kind: want redeem, got %q", stored.Kind)
	}

	// Redeem deducts: 300 - 50 = 250.
	if bal, err := store.Balance(ctx, "member-1"); err != nil || bal != 250 {
		t.Fatalf("balance after redeem: want 250, got %d (err %v)", bal, err)
	}

	// An earn row carries no reward (NULL → empty string on read-back).
	earnStored, _, err := store.RecordTransaction(ctx, earnTxn("earn-2", "member-1", 10))
	if err != nil {
		t.Fatalf("earn: %v", err)
	}
	if earnStored.Reward != "" {
		t.Fatalf("earn reward: want empty, got %q", earnStored.Reward)
	}
}
