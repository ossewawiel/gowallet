package httpapi

import (
	"net/http"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// authorizeTarget is the identity + access seam every protected handler funnels
// through. It reads the VERIFIED identity from the context (put there by the
// Authenticator middleware) and runs the pure wallet.Authorize rule against the
// account the request is trying to touch.
//
//   - member → may act only on their own account; any other target → ErrForbidden (403).
//   - admin  → may act on any account.
//
// Crucially, identity comes from the token via context — NEVER from the URL or
// body. The `target` passed in is whatever the request *names* (a path param or
// a body account_id); Authorize decides whether the caller is allowed to act on
// it. So a member token plus a body account_id naming someone else is rejected
// here (INV-13), never granting a cross-account effect.
func authorizeTarget(r *http.Request, target string) (string, error) {
	id, ok := identityFrom(r.Context())
	if !ok {
		// No verified identity on a protected route should be impossible (the
		// Authenticator gates first), but fail closed if it ever happens.
		return "", wallet.ErrForbidden
	}
	if err := wallet.Authorize(id, target); err != nil {
		return "", err
	}
	return target, nil
}

// requireAdmin gates an admin-only route. It reads the VERIFIED identity from the
// context (placed there by the Authenticator middleware) — never from the URL or
// body — and refuses anyone who isn't an admin with wallet.ErrForbidden (→ 403).
// A missing identity (should be impossible on a protected route) also fails
// closed. Reusable across admin-only operations (GET /audit now, GET /accounts
// in S7).
func requireAdmin(r *http.Request) error {
	id, ok := identityFrom(r.Context())
	if !ok || id.Role != wallet.RoleAdmin {
		return wallet.ErrForbidden
	}
	return nil
}
