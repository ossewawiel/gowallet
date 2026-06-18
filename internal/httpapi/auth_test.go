package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

const testSecret = "test-secret-do-not-use-in-prod"

// IssueToken then verifyToken must round-trip to the same Identity.
func TestIssueToken_RoundTrips(t *testing.T) {
	want := wallet.Identity{AccountID: "member-7", Role: wallet.RoleMember}
	raw, err := IssueToken(testSecret, time.Hour, want)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	got, err := verifyToken(testSecret, raw)
	if err != nil {
		t.Fatalf("verifyToken: %v", err)
	}
	if got != want {
		t.Fatalf("round-trip identity: want %+v, got %+v", want, got)
	}
}

func TestVerify_WrongSecret_Rejected(t *testing.T) {
	raw, err := IssueToken(testSecret, time.Hour, wallet.Identity{AccountID: "m", Role: wallet.RoleMember})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := verifyToken("a-different-secret", raw); err == nil {
		t.Fatalf("verify with wrong secret: want error, got nil")
	}
}

func TestVerify_Expired_Rejected(t *testing.T) {
	// Negative TTL → already expired.
	raw, err := IssueToken(testSecret, -time.Minute, wallet.Identity{AccountID: "m", Role: wallet.RoleMember})
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	if _, err := verifyToken(testSecret, raw); err == nil {
		t.Fatalf("verify expired token: want error, got nil")
	}
}

// INV-12: an alg:none token must be rejected.
func TestVerify_AlgNone_Rejected(t *testing.T) {
	tok := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub":  "member-1",
		"role": "member",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	raw, err := tok.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := verifyToken(testSecret, raw); err == nil {
		t.Fatalf("verify alg:none token: want error, got nil")
	}
}

// INV-12: a token shaped with a non-HS256 algorithm header must be refused.
func TestVerify_NonHS256_Rejected(t *testing.T) {
	// Forge a token whose header claims alg=RS256 but is "signed" with the HMAC
	// secret bytes. WithValidMethods(["HS256"]) must reject it before any
	// signature check, so the key type mismatch never even matters.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  "member-1",
		"role": "member",
		"exp":  time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["alg"] = "RS256"
	raw, err := tok.SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	if _, err := verifyToken(testSecret, raw); err == nil {
		t.Fatalf("verify non-HS256 token: want error, got nil")
	}
}

func TestAuthenticator_MissingBearer_401(t *testing.T) {
	h := Authenticator(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/accounts/member-1/balance", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("missing bearer: want 401, got %d", rec.Code)
	}
}

func TestAuthenticator_MalformedHeader_401(t *testing.T) {
	h := Authenticator(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/accounts/member-1/balance", nil)
	req.Header.Set("Authorization", "Token abc.def.ghi") // not "Bearer"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("malformed header: want 401, got %d", rec.Code)
	}
}
