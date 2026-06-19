package acceptance_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ossewawiel/gowallet/internal/httpapi"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// mintToken signs a JWT IN-PROCESS for an arbitrary identity using the same
// secret the booted test app verifies with. There is no HTTP mint endpoint any
// more (S6 removed POST /token), so tests forge tokens directly via the
// exported package signer — exactly what the real /login path calls after a
// credential check. base is unused now but kept so callers stay unchanged.
func mintToken(t *testing.T, _ /*base*/, accountID, role string) string {
	t.Helper()
	r, err := wallet.ParseRole(role)
	if err != nil {
		t.Fatalf("mintToken: bad role %q: %v", role, err)
	}
	tok, err := httpapi.IssueToken(acceptanceSecret, time.Hour, wallet.Identity{AccountID: accountID, Role: r})
	if err != nil {
		t.Fatalf("mintToken: %v", err)
	}
	return tok
}

// authGet performs GET base+path with the given bearer token.
func authGet(t *testing.T, base, path, token string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

// TestToken_IssuesUsableJWT: an in-process-minted token is accepted on a
// protected route (the same token shape /login issues after a credential check).
func TestToken_IssuesUsableJWT(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")

	token := mintToken(t, srv.URL, "member-1", "member")
	resp := authGet(t, srv.URL, "/accounts/member-1/balance", token)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("protected route with issued token: want 200, got %d", resp.StatusCode)
	}
}

func TestAuth_NoToken_ProtectedRoute_401(t *testing.T) {
	srv := bootRealApp(t)
	resp := authGet(t, srv.URL, "/accounts/member-1/balance", "")
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("protected route, no token: want 401, got %d", resp.StatusCode)
	}
}

// INV-7: member can read own balance (200) but not another's (403, not 404).
func TestAccess_MemberOwnOnly(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")

	token := mintToken(t, srv.URL, "member-1", "member")

	own := authGet(t, srv.URL, "/accounts/member-1/balance", token)
	if own.StatusCode != http.StatusOK {
		t.Fatalf("member own balance: want 200, got %d", own.StatusCode)
	}
	_ = own.Body.Close()

	other := authGet(t, srv.URL, "/accounts/member-2/balance", token)
	if other.StatusCode != http.StatusForbidden {
		t.Fatalf("member other balance: want 403, got %d", other.StatusCode)
	}
	_ = other.Body.Close()
}

// INV-8: admin can view any account and apply (earn) transactions to any account.
func TestAccess_AdminAny(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")

	admin := mintToken(t, srv.URL, "admin-1", "admin")

	// Admin earns into member-1's account.
	resp := authPostJSON(t, srv.URL+"/transactions", admin, earnBody("tx-admin", "member-1", 75))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("admin earn into member-1: want 201, got %d", resp.StatusCode)
	}
	_ = resp.Body.Close()

	for _, id := range []string{"member-1", "member-2"} {
		r := authGet(t, srv.URL, "/accounts/"+id+"/balance", admin)
		if r.StatusCode != http.StatusOK {
			t.Fatalf("admin view %s: want 200, got %d", id, r.StatusCode)
		}
		_ = r.Body.Close()
	}

	if bal := authBalance(t, srv.URL, "member-1", admin); bal != 75 {
		t.Fatalf("admin adjustment applied: want 75, got %d", bal)
	}
}

// Admin reaching a non-existent account is allowed past authz, so it surfaces
// the honest 404 from the store (not a 403, and not a 500).
func TestAccess_AdminMissingAccount_404(t *testing.T) {
	srv := bootRealApp(t)
	admin := mintToken(t, srv.URL, "admin-1", "admin")
	resp := authGet(t, srv.URL, "/accounts/ghost/balance", admin)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("admin missing account: want 404, got %d", resp.StatusCode)
	}
}

// INV-12: a forged alg:none token is rejected at the door (401).
func TestAuth_AlgConfusion_Rejected(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")

	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub":  "member-1",
		"role": "member",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	forged, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("forge alg:none: %v", err)
	}

	resp := authGet(t, srv.URL, "/accounts/member-1/balance", forged)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("forged alg:none token: want 401, got %d", resp.StatusCode)
	}
}

// INV-13: identity comes from the token only. A member's token + a body
// account_id naming someone else must NOT touch the other account.
func TestAuth_IdentityFromTokenOnly(t *testing.T) {
	srv := bootRealApp(t)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")

	token := mintToken(t, srv.URL, "member-1", "member")

	// member-1's token, but the body claims account_id = member-2.
	resp := authPostJSON(t, srv.URL+"/transactions", token, earnBody("tx-evil", "member-2", 500))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK {
		t.Fatalf("cross-account write via body account_id: want rejection, got %d", resp.StatusCode)
	}

	// member-2 must be untouched.
	admin := mintToken(t, srv.URL, "admin-1", "admin")
	if bal := authBalance(t, srv.URL, "member-2", admin); bal != 0 {
		t.Fatalf("member-2 balance after spoof attempt: want 0, got %d", bal)
	}
}
