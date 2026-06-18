package wallet

import "errors"

// ErrForbidden — the caller is authenticated but not allowed to act on the
// target account (a member reaching for someone else's account). The api layer
// maps this to 403. It is deliberately distinct from ErrNotFound: leaking
// "this account exists, you just can't see it" is fine here because the caller
// already proved who they are with a valid token.
var ErrForbidden = errors.New("forbidden")

// Role is the access level carried in a verified token. Identity comes from the
// token only — never from a URL or body — so these types live in wallet (the
// core) and the authorization rule can own them.
type Role string

const (
	// RoleMember may act only on its own account.
	RoleMember Role = "member"
	// RoleAdmin may act on any account, including adjustments.
	RoleAdmin Role = "admin"
)

// Identity is the verified caller: which account they are (sub) and their role.
// It is built by the transport layer from a verified JWT and handed to
// Authorize — the pure rule below.
type Identity struct {
	AccountID string
	Role      Role
}

// ParseRole turns an untrusted role string (from the /token request body) into
// a Role, rejecting anything unknown with ErrInvalidInput (→ 422 at the edge).
func ParseRole(s string) (Role, error) {
	switch Role(s) {
	case RoleMember:
		return RoleMember, nil
	case RoleAdmin:
		return RoleAdmin, nil
	default:
		return "", ErrInvalidInput
	}
}

// Authorize is the access rule, pure and HTTP-free so it unit-tests in
// isolation:
//   - admin  → may act on any account.
//   - member → may act only on their own account; anything else is ErrForbidden.
func Authorize(id Identity, targetAccountID string) error {
	if id.Role == RoleAdmin {
		return nil
	}
	if id.AccountID == targetAccountID {
		return nil
	}
	return ErrForbidden
}
