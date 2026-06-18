package acceptance_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

// loginResp is the decoded body of a successful POST /login.
type loginResp struct {
	Token     string `json:"token"`
	TokenType string `json:"token_type"`
	ExpiresIn int    `json:"expires_in"`
}

// roleFromToken parses the (unverified) role claim out of a JWT — enough for an
// acceptance assertion that the issued role matches the stored account.
func roleFromToken(t *testing.T, raw string) string {
	t.Helper()
	claims := jwt.MapClaims{}
	if _, _, err := jwt.NewParser().ParseUnverified(raw, claims); err != nil {
		t.Fatalf("parse token: %v", err)
	}
	role, _ := claims["role"].(string)
	return role
}

// TestLogin_ValidCredential_IssuesToken (INV-14) — the seeded member logs in,
// gets a 200 + token whose role is the stored 'member', and the token works on
// a protected route.
func TestLogin_ValidCredential_IssuesToken(t *testing.T) {
	srv := bootRealApp(t)

	resp := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "member-123", "secret": "demo-member-pw",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login valid: want 200, got %d", resp.StatusCode)
	}
	var body loginResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode login: %v", err)
	}
	if body.Token == "" || body.TokenType != "Bearer" || body.ExpiresIn <= 0 {
		t.Fatalf("login response malformed: %+v", body)
	}
	if r := roleFromToken(t, body.Token); r != "member" {
		t.Fatalf("issued role: want member (from store), got %q", r)
	}

	// The token must work on a protected route for member-123's own data.
	bal := authGet(t, srv.URL, "/accounts/member-123/balance", body.Token)
	defer func() { _ = bal.Body.Close() }()
	if bal.StatusCode != http.StatusOK {
		t.Fatalf("seeded member balance with login token: want 200, got %d", bal.StatusCode)
	}
}

// TestLogin_BadCredential_401 (INV-15) — a wrong secret gets a 401
// invalid_credentials with NO token in the body.
func TestLogin_BadCredential_401(t *testing.T) {
	srv := bootRealApp(t)

	resp := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "member-123", "secret": "wrong-pw",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad credential: want 401, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if _, hasToken := body["token"]; hasToken {
		t.Fatalf("error body must not carry a token: %v", body)
	}
	errObj, _ := body["error"].(map[string]any)
	if errObj == nil || errObj["code"] != "invalid_credentials" {
		t.Fatalf("error code: want invalid_credentials, got %v", body)
	}
}

// TestLogin_UnknownAccount_SameResponse (INV-15) — an unknown account returns a
// response byte-identical in shape to the wrong-secret case (same status, same
// code, no token) so accounts can't be enumerated.
func TestLogin_UnknownAccount_SameResponse(t *testing.T) {
	srv := bootRealApp(t)

	wrong := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "member-123", "secret": "wrong-pw",
	})
	defer func() { _ = wrong.Body.Close() }()
	ghost := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "ghost-999", "secret": "anything",
	})
	defer func() { _ = ghost.Body.Close() }()

	if wrong.StatusCode != http.StatusUnauthorized || ghost.StatusCode != http.StatusUnauthorized {
		t.Fatalf("both should be 401; wrong=%d ghost=%d", wrong.StatusCode, ghost.StatusCode)
	}

	decodeCode := func(r *http.Response) (string, bool) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode: %v", err)
		}
		_, hasToken := body["token"]
		errObj, _ := body["error"].(map[string]any)
		code, _ := errObj["code"].(string)
		return code, hasToken
	}
	wCode, wTok := decodeCode(wrong)
	gCode, gTok := decodeCode(ghost)
	if wCode != gCode {
		t.Fatalf("enumeration leak: wrong-secret code %q != unknown-account code %q", wCode, gCode)
	}
	if wTok || gTok {
		t.Fatalf("neither response may carry a token (wrong=%v ghost=%v)", wTok, gTok)
	}
}

// TestLogin_RoleFromStore_NotRequest (INV-16) — the admin logs in and gets a
// role=admin token (from the store); and a body carrying a `role` field is
// rejected at the schema edge (additionalProperties:false → 400).
func TestLogin_RoleFromStore_NotRequest(t *testing.T) {
	srv := bootRealApp(t)

	resp := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "admin-001", "secret": "demo-admin-pw",
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin login: want 200, got %d", resp.StatusCode)
	}
	var body loginResp
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if r := roleFromToken(t, body.Token); r != "admin" {
		t.Fatalf("issued role: want admin (from store), got %q", r)
	}

	// A client can never ask for a role: an extra `role` field is rejected.
	bad := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "member-123", "secret": "demo-member-pw", "role": "admin",
	})
	defer func() { _ = bad.Body.Close() }()
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("login with extra role field: want 400 (additionalProperties:false), got %d", bad.StatusCode)
	}
}

// TestAccounts_SecretNeverReturned (INV-17) — creating an account WITH a secret
// returns a 201 body with no secret/password_hash, and GET likewise.
func TestAccounts_SecretNeverReturned(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)

	created := authPostJSON(t, srv.URL+"/accounts", admin, map[string]any{
		"account_id": "newbie-1", "name": "Newbie", "secret": "joinpw-123",
	})
	defer func() { _ = created.Body.Close() }()
	if created.StatusCode != http.StatusCreated {
		t.Fatalf("create account with secret: want 201, got %d", created.StatusCode)
	}
	assertNoSecretFields(t, created)

	got := authGet(t, srv.URL, "/accounts/newbie-1", admin)
	defer func() { _ = got.Body.Close() }()
	if got.StatusCode != http.StatusOK {
		t.Fatalf("get account: want 200, got %d", got.StatusCode)
	}
	assertNoSecretFields(t, got)

	// And the new secret actually works as a login (proves it was stored).
	login := postJSON(t, srv.URL+"/login", map[string]any{
		"account_id": "newbie-1", "secret": "joinpw-123",
	})
	defer func() { _ = login.Body.Close() }()
	if login.StatusCode != http.StatusOK {
		t.Fatalf("login with the secret set at creation: want 200, got %d", login.StatusCode)
	}
}

// assertNoSecretFields fails if the JSON body leaks secret/password_hash.
func assertNoSecretFields(t *testing.T, resp *http.Response) {
	t.Helper()
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode account body: %v", err)
	}
	for _, k := range []string{"secret", "password_hash", "passwordHash"} {
		if _, ok := body[k]; ok {
			t.Fatalf("account body leaks %q: %v", k, body)
		}
	}
}
