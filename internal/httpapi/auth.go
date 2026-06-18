package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// ctxKey is an unexported type so no other package can collide with our context
// key — the only request-specific state we share is via r.Context().
type ctxKey int

const identityKey ctxKey = iota

// claims is the JWT payload we sign and verify. sub = account_id, role = the
// access level. RegisteredClaims gives us exp/iat handling for free.
type claims struct {
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// IssueToken signs an HS256 JWT for the given identity, valid for ttl. This is
// the demo token mint behind POST /token — a real system would authenticate a
// credential first, then call this.
func IssueToken(secret string, ttl time.Duration, id wallet.Identity) (string, error) {
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		Role: string(id.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   id.AccountID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	})
	return tok.SignedString([]byte(secret))
}

// verifyToken parses and validates a raw JWT and returns the carried Identity.
// The algorithm is PINNED to HS256 via WithValidMethods — that single option
// kills the alg:none and RS↔HS confusion attacks before any signature check.
func verifyToken(secret, raw string) (wallet.Identity, error) {
	var c claims
	_, err := jwt.ParseWithClaims(raw, &c, func(_ *jwt.Token) (any, error) {
		return []byte(secret), nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return wallet.Identity{}, err
	}
	role, err := wallet.ParseRole(c.Role)
	if err != nil {
		return wallet.Identity{}, err
	}
	if c.Subject == "" {
		return wallet.Identity{}, errors.New("token missing subject")
	}
	return wallet.Identity{AccountID: c.Subject, Role: role}, nil
}

// Authenticator is the verification middleware: it pulls the Bearer token,
// verifies it, and stashes the resulting Identity in the request context. Any
// failure (missing/malformed header, bad signature, expired, wrong alg) is a
// 401 — these never reach the wallet core.
func Authenticator(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearerToken(r)
			if !ok {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "missing or malformed bearer token")
				return
			}
			id, err := verifyToken(secret, raw)
			if err != nil {
				writeError(w, r, http.StatusUnauthorized, "unauthorized", "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), identityKey, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerToken extracts the raw token from a well-formed "Authorization: Bearer
// <token>" header. Anything else (missing, wrong scheme, empty) → not ok.
func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	tok := strings.TrimSpace(h[len(prefix):])
	if tok == "" {
		return "", false
	}
	return tok, true
}

// identityFrom returns the verified Identity placed by Authenticator. ok is
// false on public routes (no auth ran).
func identityFrom(ctx context.Context) (wallet.Identity, bool) {
	id, ok := ctx.Value(identityKey).(wallet.Identity)
	return id, ok
}
