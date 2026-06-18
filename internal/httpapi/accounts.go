package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// CreateAccount handles POST /accounts → 201 + Location, or 409 if the id is
// taken, 400 on a malformed body.
func (s *server) CreateAccount(w http.ResponseWriter, r *http.Request) {
	var body gen.NewAccount
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	if body.AccountId == "" || body.Name == "" {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "account_id and name are required")
		return
	}

	if err := s.wallet.CreateAccount(r.Context(), wallet.Account{ID: body.AccountId, Name: body.Name}); err != nil {
		writeDomainError(w, r, err)
		return
	}
	// Read it back so the 201 body carries the DB-assigned created_at.
	acct, err := s.wallet.GetAccount(r.Context(), body.AccountId)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	w.Header().Set("Location", "/accounts/"+acct.ID)
	writeJSON(w, http.StatusCreated, gen.Account{
		AccountId: acct.ID,
		Name:      acct.Name,
		CreatedAt: acct.CreatedAt,
	})
}

// GetAccount handles GET /accounts/{id} → 200 or 404.
func (s *server) GetAccount(w http.ResponseWriter, r *http.Request, accountID string) {
	id, err := authorizeTarget(r, accountID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	acct, err := s.wallet.GetAccount(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.Account{
		AccountId: acct.ID,
		Name:      acct.Name,
		CreatedAt: acct.CreatedAt,
	})
}

// GetBalance handles GET /accounts/{id}/balance → 200 or 404.
func (s *server) GetBalance(w http.ResponseWriter, r *http.Request, accountID string) {
	id, err := authorizeTarget(r, accountID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	bal, err := s.wallet.Balance(r.Context(), id)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, gen.Balance{AccountId: id, Balance: bal})
}

// CreateTransaction handles POST /transactions → 201 (new earn) / 200
// (idempotent replay) / 404 (unknown account) / 400 (bad body or kind≠earn).
func (s *server) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	var body gen.NewTransaction
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}
	if body.Ref == "" || body.AccountId == "" || body.Points < 1 || body.OccurredAt.IsZero() {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "ref, account_id, points (>=1) and occurred_at are required")
		return
	}
	// earn and spend are the only legal directions (S2 widened the enum).
	// kind selects the service method; anything else dies at the edge as 400.
	if body.Kind != "earn" && body.Kind != "spend" {
		writeError(w, r, http.StatusBadRequest, "invalid_kind", "kind must be 'earn' or 'spend'")
		return
	}

	id, err := authorizeTarget(r, body.AccountId)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}
	txn := wallet.Transaction{
		Ref:        body.Ref,
		AccountID:  id,
		Points:     body.Points,
		OccurredAt: body.OccurredAt,
	}
	var (
		stored  wallet.Transaction
		created bool
	)
	switch body.Kind {
	case "earn":
		stored, created, err = s.wallet.RecordEarn(r.Context(), txn)
	case "spend":
		stored, created, err = s.wallet.RecordSpend(r.Context(), txn)
	}
	if err != nil {
		if errors.Is(err, wallet.ErrNotFound) {
			writeError(w, r, http.StatusNotFound, "account_not_found", "account_id does not exist")
			return
		}
		writeDomainError(w, r, err)
		return
	}

	status := http.StatusOK // idempotent replay
	if created {
		status = http.StatusCreated
	}
	writeJSON(w, status, gen.Transaction{
		Ref:        stored.Ref,
		AccountId:  stored.AccountID,
		Kind:       gen.TransactionKind(stored.Kind),
		Points:     stored.Points,
		OccurredAt: stored.OccurredAt,
	})
}

// writeJSON encodes v as the response body with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
