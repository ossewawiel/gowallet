package acceptance_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// auditWire mirrors the AuditEntry schema on the wire.
type auditWire struct {
	ID        int64     `json:"id"`
	Ref       string    `json:"ref"`
	AccountID string    `json:"account_id"`
	Kind      string    `json:"kind"`
	Points    int64     `json:"points"`
	Outcome   string    `json:"outcome"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}

// seedAudit appends an audit row directly through the store (there is no HTTP
// write path for audit in S4).
func seedAudit(t *testing.T, store auditSeeder, ref, accountID string, outcome wallet.AuditOutcome, reason string) {
	t.Helper()
	if _, err := store.AppendAudit(context.Background(), wallet.AuditEntry{
		Ref: ref, AccountID: accountID, Kind: "earn", Points: 10,
		Outcome: outcome, Reason: reason,
	}); err != nil {
		t.Fatalf("seed audit %s: %v", ref, err)
	}
}

// auditSeeder is the slice of the store the audit acceptance tests need.
type auditSeeder interface {
	AppendAudit(ctx context.Context, e wallet.AuditEntry) (wallet.AuditEntry, error)
}

// TestListAudit_AdminOnly (INV-21) — member → 403; admin → 200.
func TestListAudit_AdminOnly(t *testing.T) {
	srv, _ := bootRealAppWithStore(t)

	memberTok := mintToken(t, srv.URL, "member-1", "member")
	resp := authGet(t, srv.URL, "/audit", memberTok)
	memberStatus := resp.StatusCode
	_ = resp.Body.Close()
	if memberStatus != http.StatusForbidden {
		t.Fatalf("member GET /audit: want 403, got %d", memberStatus)
	}

	adminResp := authGet(t, srv.URL, "/audit", adminToken(t, srv.URL))
	defer func() { _ = adminResp.Body.Close() }()
	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("admin GET /audit: want 200, got %d", adminResp.StatusCode)
	}
}

// TestListAudit_RecordsShape (INV-21) — seeded rows come back with
// outcome + reason + created_at; ?account_id= filters; newest-first.
func TestListAudit_RecordsShape(t *testing.T) {
	srv, store := bootRealAppWithStore(t)

	seedAudit(t, store, "m1-a", "member-1", wallet.OutcomeAccepted, "ok")
	seedAudit(t, store, "m2-a", "member-2", wallet.OutcomeRejected, "account not found")
	seedAudit(t, store, "m1-b", "member-1", wallet.OutcomeDuplicate, "duplicate ref")

	admin := adminToken(t, srv.URL)

	// Full log — newest-first, all three rows, each shaped.
	all := getAudit(t, srv.URL, "", admin)
	if len(all) != 3 {
		t.Fatalf("full log: want 3, got %d", len(all))
	}
	if all[0].Ref != "m1-b" || all[2].Ref != "m1-a" {
		t.Fatalf("newest-first: got order %s, %s, %s", all[0].Ref, all[1].Ref, all[2].Ref)
	}
	for _, e := range all {
		if e.Outcome == "" || e.Reason == "" || e.CreatedAt.IsZero() {
			t.Fatalf("row %s: want outcome+reason+created_at, got %+v", e.Ref, e)
		}
	}

	// Filtered — only member-1's two rows, no leak.
	filtered := getAudit(t, srv.URL, "member-1", admin)
	if len(filtered) != 2 {
		t.Fatalf("filtered: want 2, got %d", len(filtered))
	}
	for _, e := range filtered {
		if e.AccountID != "member-1" {
			t.Fatalf("filter leak: row for %q", e.AccountID)
		}
	}
}

// TestListAudit_NoToken_401 — no token → 401.
func TestListAudit_NoToken_401(t *testing.T) {
	srv, _ := bootRealAppWithStore(t)
	resp := authGet(t, srv.URL, "/audit", "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token GET /audit: want 401, got %d", resp.StatusCode)
	}
}

// getAudit GETs /audit (optionally filtered) with the token and decodes the array.
func getAudit(t *testing.T, base, accountID, token string) []auditWire {
	t.Helper()
	path := "/audit"
	if accountID != "" {
		path += "?account_id=" + accountID
	}
	resp := authGet(t, base, path, token)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s: want 200, got %d", path, resp.StatusCode)
	}
	var out []auditWire
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode audit log: %v", err)
	}
	return out
}
