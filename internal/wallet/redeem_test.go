package wallet_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// redeem builds a redeem-shaped Transaction for the service tests. The service
// forces Kind=redeem itself; we set it here too so a forcing test can prove the
// service overrides a wrong incoming kind.
func redeem(ref, id, reward string, pts int64) wallet.Transaction {
	return wallet.Transaction{
		Ref:        ref,
		AccountID:  id,
		Kind:       wallet.KindRedeem,
		Points:     pts,
		Reward:     reward,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
}

// A redeem with an empty reward is invalid — reward records WHAT was bought, so
// it's required (unlike earn/spend, which carry no reward).
func TestRecordRedeem_RequiresReward(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	_, _, err := svc.RecordRedeem(context.Background(), redeem("rdm-1", "member-1", "", 50))
	if !errors.Is(err, wallet.ErrInvalidInput) {
		t.Fatalf("empty reward: want ErrInvalidInput, got %v", err)
	}
}

// RecordRedeem must force Kind=redeem regardless of what the caller passed in —
// the service owns the direction, not the request body.
func TestRecordRedeem_ForcesKindRedeem(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	in := wallet.Transaction{Ref: "rdm-1", AccountID: "member-1", Kind: wallet.KindEarn,
		Points: 10, Reward: "R10 voucher",
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC)}
	got, _, err := svc.RecordRedeem(context.Background(), in)
	if err != nil {
		t.Fatalf("RecordRedeem: %v", err)
	}
	if got.Kind != wallet.KindRedeem {
		t.Fatalf("kind: want redeem, got %q", got.Kind)
	}
}

func TestRecordRedeem_RejectsNonPositivePoints(t *testing.T) {
	svc, _ := newService(t)
	mustAccount(t, svc, "member-1")

	_, _, err := svc.RecordRedeem(context.Background(), redeem("rdm-1", "member-1", "R0", 0))
	if !errors.Is(err, wallet.ErrInvalidInput) {
		t.Fatalf("err: want ErrInvalidInput, got %v", err)
	}
}
