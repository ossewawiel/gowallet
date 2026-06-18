package wallet

import (
	"errors"
	"testing"
)

// INV-7: a member may act only on their own account; any other target → ErrForbidden.
func TestAuthorize_MemberOwnAccount_Allowed(t *testing.T) {
	id := Identity{AccountID: "member-1", Role: RoleMember}
	if err := Authorize(id, "member-1"); err != nil {
		t.Fatalf("member on own account: want nil, got %v", err)
	}
}

func TestAuthorize_MemberOtherAccount_Forbidden(t *testing.T) {
	id := Identity{AccountID: "member-1", Role: RoleMember}
	err := Authorize(id, "member-2")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("member on another account: want ErrForbidden, got %v", err)
	}
}

// INV-8: an admin may act on any account.
func TestAuthorize_AdminAnyAccount_Allowed(t *testing.T) {
	id := Identity{AccountID: "admin-1", Role: RoleAdmin}
	for _, target := range []string{"member-1", "member-2", "admin-1"} {
		if err := Authorize(id, target); err != nil {
			t.Fatalf("admin on %s: want nil, got %v", target, err)
		}
	}
}

func TestRole_Parse_RejectsUnknown(t *testing.T) {
	if _, err := ParseRole("member"); err != nil {
		t.Fatalf("ParseRole(member): want nil, got %v", err)
	}
	if _, err := ParseRole("admin"); err != nil {
		t.Fatalf("ParseRole(admin): want nil, got %v", err)
	}
	if _, err := ParseRole("wizard"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("ParseRole(wizard): want ErrInvalidInput, got %v", err)
	}
}
