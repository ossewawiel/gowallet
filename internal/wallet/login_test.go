package wallet_test

import (
	"context"
	"errors"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/ossewawiel/gowallet/internal/wallet"
)

// credRepo is an in-memory AccountRepository that also serves credentials, so
// the login rule can be unit-tested with no database.
type credRepo struct {
	*fakeRepo
	// creds maps account_id → (bcrypt hash, role). A missing entry ⇒ ErrNotFound.
	creds map[string]struct {
		hash string
		role wallet.Role
	}
}

func newCredRepo() *credRepo {
	return &credRepo{
		fakeRepo: newFakeRepo(),
		creds: map[string]struct {
			hash string
			role wallet.Role
		}{},
	}
}

func (c *credRepo) GetCredential(_ context.Context, id string) (string, wallet.Role, error) {
	cred, ok := c.creds[id]
	if !ok {
		return "", "", wallet.ErrNotFound
	}
	return cred.hash, cred.role, nil
}

// seedCred stores a bcrypt hash of secret under id with the given role.
func (c *credRepo) seedCred(t *testing.T, id, secret string, role wallet.Role) {
	t.Helper()
	h, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	c.creds[id] = struct {
		hash string
		role wallet.Role
	}{string(h), role}
}

func newLoginService(t *testing.T) (*wallet.WalletService, *credRepo) {
	t.Helper()
	repo := newCredRepo()
	return wallet.NewWalletService(repo, repo), repo
}

// TestLogin_ValidCredential_ReturnsStoredRole — a correct secret returns the
// identity carrying the STORED role (INV-14, INV-16 at the domain seam).
func TestLogin_ValidCredential_ReturnsStoredRole(t *testing.T) {
	svc, repo := newLoginService(t)
	repo.seedCred(t, "admin-001", "demo-admin-pw", wallet.RoleAdmin)

	id, err := svc.Login(context.Background(), "admin-001", "demo-admin-pw")
	if err != nil {
		t.Fatalf("Login valid: unexpected err %v", err)
	}
	if id.AccountID != "admin-001" {
		t.Fatalf("identity account: want admin-001, got %q", id.AccountID)
	}
	if id.Role != wallet.RoleAdmin {
		t.Fatalf("identity role: want admin (from store), got %q", id.Role)
	}
}

// TestLogin_WrongSecret_Invalid — a wrong secret returns ErrInvalidCredentials.
func TestLogin_WrongSecret_Invalid(t *testing.T) {
	svc, repo := newLoginService(t)
	repo.seedCred(t, "member-123", "demo-member-pw", wallet.RoleMember)

	_, err := svc.Login(context.Background(), "member-123", "wrong-pw")
	if !errors.Is(err, wallet.ErrInvalidCredentials) {
		t.Fatalf("wrong secret: want ErrInvalidCredentials, got %v", err)
	}
}

// TestLogin_UnknownAccount_Invalid — an absent account returns the SAME error
// value as a wrong secret (no enumeration).
func TestLogin_UnknownAccount_Invalid(t *testing.T) {
	svc, _ := newLoginService(t)

	_, err := svc.Login(context.Background(), "ghost-999", "anything")
	if !errors.Is(err, wallet.ErrInvalidCredentials) {
		t.Fatalf("unknown account: want ErrInvalidCredentials, got %v", err)
	}
}

// TestCreateAccount_SecretTooLong_InvalidInput — bcrypt rejects secrets over 72
// bytes; that's a client error (→ 400), never a 500. The service maps it to
// ErrInvalidInput rather than leaking the raw bcrypt error.
func TestCreateAccount_SecretTooLong_InvalidInput(t *testing.T) {
	svc, _ := newLoginService(t)
	longSecret := ""
	for i := 0; i < 100; i++ {
		longSecret += "x"
	}
	err := svc.CreateAccount(context.Background(), wallet.Account{ID: "m-1", Name: "T"}, longSecret)
	if !errors.Is(err, wallet.ErrInvalidInput) {
		t.Fatalf("over-long secret: want ErrInvalidInput, got %v", err)
	}
}

// TestLogin_NoStoredSecret_Invalid — an account with a NULL/empty hash can't log
// in; same ErrInvalidCredentials.
func TestLogin_NoStoredSecret_Invalid(t *testing.T) {
	svc, repo := newLoginService(t)
	repo.creds["secretless"] = struct {
		hash string
		role wallet.Role
	}{"", wallet.RoleMember}

	_, err := svc.Login(context.Background(), "secretless", "anything")
	if !errors.Is(err, wallet.ErrInvalidCredentials) {
		t.Fatalf("secret-less account: want ErrInvalidCredentials, got %v", err)
	}
}
