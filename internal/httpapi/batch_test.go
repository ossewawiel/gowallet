package httpapi

import (
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// TestParseRow_Valid — a well-formed CSV record maps to the right
// wallet.Transaction (kind, points, occurred_at all parsed).
func TestParseRow_Valid(t *testing.T) {
	rec := []string{"tx-001", "member-123", "earn", "150", "2024-06-01T10:00:00Z"}
	txn, err := parseRow(rec)
	if err != nil {
		t.Fatalf("parseRow valid: unexpected error %v", err)
	}
	want := wallet.Transaction{
		Ref:        "tx-001",
		AccountID:  "member-123",
		Kind:       wallet.KindEarn,
		Points:     150,
		OccurredAt: time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
	}
	if txn.Ref != want.Ref || txn.AccountID != want.AccountID || txn.Kind != want.Kind ||
		txn.Points != want.Points || !txn.OccurredAt.Equal(want.OccurredAt) {
		t.Fatalf("parseRow: want %+v, got %+v", want, txn)
	}
}

// TestParseRow_ValidSpend — kind=spend maps to KindSpend.
func TestParseRow_ValidSpend(t *testing.T) {
	rec := []string{"tx-002", "member-123", "spend", "40", "2024-06-01T10:00:00Z"}
	txn, err := parseRow(rec)
	if err != nil {
		t.Fatalf("parseRow valid spend: unexpected error %v", err)
	}
	if txn.Kind != wallet.KindSpend {
		t.Fatalf("parseRow spend: want kind spend, got %q", txn.Kind)
	}
}

// TestParseRow_Rejects — each malformed record returns a parse error with a
// stable reason string (the reason becomes the audit row's reason).
func TestParseRow_Rejects(t *testing.T) {
	cases := []struct {
		name   string
		rec    []string
		reason string
	}{
		{"bad kind", []string{"tx", "acc", "gift", "10", "2024-06-01T10:00:00Z"}, "invalid kind"},
		{"non-integer points", []string{"tx", "acc", "earn", "ten", "2024-06-01T10:00:00Z"}, "invalid points"},
		{"points below one", []string{"tx", "acc", "earn", "0", "2024-06-01T10:00:00Z"}, "invalid points"},
		{"negative points", []string{"tx", "acc", "earn", "-5", "2024-06-01T10:00:00Z"}, "invalid points"},
		{"bad timestamp", []string{"tx", "acc", "earn", "10", "not-a-time"}, "invalid occurred_at"},
		{"too few columns", []string{"tx", "acc", "earn", "10"}, "malformed row"},
		{"too many columns", []string{"tx", "acc", "earn", "10", "2024-06-01T10:00:00Z", "extra"}, "malformed row"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseRow(tc.rec)
			if err == nil {
				t.Fatalf("parseRow %q: want error, got nil", tc.name)
			}
			if err.Error() != tc.reason {
				t.Fatalf("parseRow %q: want reason %q, got %q", tc.name, tc.reason, err.Error())
			}
		})
	}
}

// TestClassifyOutcome — maps (created, err) to the audit outcome, reason, and
// summary bucket per the slice's classification table.
func TestClassifyOutcome(t *testing.T) {
	cases := []struct {
		name    string
		created bool
		err     error
		outcome wallet.AuditOutcome
		reason  string
		bucket  bucket
	}{
		{"created", true, nil, wallet.OutcomeAccepted, "ok", bucketAccepted},
		{"replay", false, nil, wallet.OutcomeDuplicate, "duplicate ref", bucketDuplicate},
		{"unknown account", false, wallet.ErrNotFound, wallet.OutcomeRejected, "account not found", bucketRejected},
		{"insufficient", false, wallet.ErrInsufficientBalance, wallet.OutcomeRejected, "insufficient balance", bucketRejected},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			outcome, reason, b := classifyOutcome(tc.created, tc.err)
			if outcome != tc.outcome || reason != tc.reason || b != tc.bucket {
				t.Fatalf("classifyOutcome(%v,%v): want (%s,%q,%v), got (%s,%q,%v)",
					tc.created, tc.err, tc.outcome, tc.reason, tc.bucket, outcome, reason, b)
			}
		})
	}
}
