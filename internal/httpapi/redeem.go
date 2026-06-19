package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// RedeemPoints handles POST /accounts/{account_id}/redeem — a member redemption
// (member-own / admin-any). It returns 201 on a new redemption, 200 on an
// idempotent replay of a known ref, 400 on a bad body, 403 cross-account member,
// 404 unknown account, 409 if the redeem would drive the balance below zero.
//
// Identity comes from the verified token via authorizeTarget — NEVER the path
// param. A member token plus someone else's {account_id} is rejected here (403,
// INV-27), so the path names only the TARGET, not who's acting.
func (s *server) RedeemPoints(w http.ResponseWriter, r *http.Request, accountID string) {
	var body gen.NewRedemption
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "malformed JSON body")
		return
	}

	// authorize before touching the store: a cross-account member gets 403, not a
	// 404 that would leak whether the target account exists.
	id, err := authorizeTarget(r, accountID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	stored, created, err := s.wallet.RecordRedeem(r.Context(), wallet.Transaction{
		Ref:        body.Ref,
		AccountID:  id,
		Points:     body.Points,
		Reward:     body.Reward,
		OccurredAt: body.OccurredAt,
	})
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
	writeJSON(w, status, gen.Redemption{
		Ref:        stored.Ref,
		AccountId:  stored.AccountID,
		Kind:       gen.Redeem,
		Points:     stored.Points,
		Reward:     stored.Reward,
		OccurredAt: stored.OccurredAt,
	})
}
