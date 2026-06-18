package httpapi

import (
	"net/http"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// ListAudit handles GET /audit → 200 (admin) with the audit log newest-first, or
// 403 for a non-admin. It is admin-only: the spec's bearerAuth scheme can't
// express a role, so the enforcement lives here. The optional ?account_id=
// filter is validated against the spec pattern at the edge (a bad value → 400
// from the validator before we run).
func (s *server) ListAudit(w http.ResponseWriter, r *http.Request, params gen.ListAuditParams) {
	if err := requireAdmin(r); err != nil {
		writeDomainError(w, r, err)
		return
	}

	accountID := ""
	if params.AccountId != nil {
		accountID = *params.AccountId
	}

	entries, err := s.audit.List(r.Context(), accountID)
	if err != nil {
		writeDomainError(w, r, err)
		return
	}

	out := make(gen.AuditLog, 0, len(entries))
	for _, e := range entries {
		out = append(out, auditToWire(e))
	}
	writeJSON(w, http.StatusOK, out)
}

// auditToWire maps a domain AuditEntry to its OpenAPI wire shape.
func auditToWire(e wallet.AuditEntry) gen.AuditEntry {
	return gen.AuditEntry{
		Id:        e.ID,
		Ref:       e.Ref,
		AccountId: e.AccountID,
		Kind:      e.Kind,
		Points:    e.Points,
		Outcome:   gen.AuditEntryOutcome(e.Outcome),
		Reason:    e.Reason,
		CreatedAt: e.CreatedAt,
	}
}
