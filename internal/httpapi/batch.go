package httpapi

import (
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ossewawiel/gowallet/internal/httpapi/gen"
	"github.com/ossewawiel/gowallet/internal/wallet"
)

// expectedHeader is the CSV header every batch file must start with. We match on
// it so a file with the wrong columns (or no header at all) fails fast as a 400
// — a broken *upload*, distinct from a valid file whose individual rows are bad
// (those are data → 200 + counted in `rejected`).
var expectedHeader = []string{"ref", "account_id", "kind", "points", "occurred_at"}

// bucket is which summary tally a resolved row lands in.
type bucket int

const (
	bucketAccepted bucket = iota
	bucketRejected
	bucketDuplicate
)

// IngestBatch handles POST /batch — admin-only CSV ingestion. Each data row is
// driven through the SAME earn/spend services as a single POST /transactions,
// so idempotency (transactions.ref UNIQUE), the no-negative spend guard, and the
// single-writer serialisation all apply with no special-casing. Every row's
// outcome is appended to the S4 audit trail AFTER its transaction resolves
// (never inside it — an audit failure must never roll back committed points).
//
// Rejections (bad row / unknown account / would-go-negative) are DATA, not HTTP
// errors: they still return 200 and are counted in `rejected`. Only a broken
// upload (missing file part, unreadable body, absent/unrecognised header) is a
// 400.
func (s *server) IngestBatch(w http.ResponseWriter, r *http.Request) {
	if err := requireAdmin(r); err != nil {
		writeDomainError(w, r, err)
		return
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, r, http.StatusBadRequest, "invalid_input", "missing multipart 'file' part")
		return
	}
	defer func() { _ = file.Close() }()

	reader := csv.NewReader(file)
	reader.FieldsPerRecord = -1 // we validate arity per row in parseRow

	header, err := reader.Read()
	if err != nil || !sameHeader(header, expectedHeader) {
		writeError(w, r, http.StatusBadRequest, "invalid_input",
			"unreadable or unrecognised CSV header — expected ref,account_id,kind,points,occurred_at")
		return
	}

	var sum gen.BatchSummary
	for {
		rec, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			// A structurally broken record (e.g. an unterminated quote) is one
			// rejected data row, not a failed request.
			sum.Processed++
			sum.Rejected++
			s.recordAudit(r, wallet.AuditEntry{
				Ref: "", AccountID: "", Kind: "", Points: 0,
				Outcome: wallet.OutcomeRejected, Reason: "malformed row",
			})
			continue
		}
		sum.Processed++
		s.processRow(r, rec, &sum)
	}

	writeJSON(w, http.StatusOK, sum)
}

// processRow parses one record, drives it through the matching service, then
// classifies + audits + tallies the outcome.
func (s *server) processRow(r *http.Request, rec []string, sum *gen.BatchSummary) {
	txn, perr := parseRow(rec)
	if perr != nil {
		sum.Rejected++
		s.recordAudit(r, wallet.AuditEntry{
			Ref: refOf(rec), AccountID: acctOf(rec), Kind: kindOf(rec), Points: 0,
			Outcome: wallet.OutcomeRejected, Reason: perr.Error(),
		})
		return
	}

	var (
		created bool
		err     error
	)
	switch txn.Kind {
	case wallet.KindEarn:
		_, created, err = s.wallet.RecordEarn(r.Context(), txn)
	case wallet.KindSpend:
		_, created, err = s.wallet.RecordSpend(r.Context(), txn)
	}

	outcome, reason, b := classifyOutcome(created, err)
	switch b {
	case bucketAccepted:
		sum.Accepted++
	case bucketDuplicate:
		sum.Duplicates++
	case bucketRejected:
		sum.Rejected++
	}
	s.recordAudit(r, wallet.AuditEntry{
		Ref: txn.Ref, AccountID: txn.AccountID, Kind: string(txn.Kind), Points: txn.Points,
		Outcome: outcome, Reason: reason,
	})
}

// recordAudit appends one attempt to the audit trail, OFF the money path. A
// failure to audit is logged-and-swallowed via the recoverer at worst; here we
// simply ignore the error so a flaky audit write can't fail an otherwise-applied
// batch (the points are already committed).
func (s *server) recordAudit(r *http.Request, e wallet.AuditEntry) {
	_, _ = s.audit.Record(r.Context(), e)
}

// classifyOutcome maps a service result (created flag + error) to the audit
// outcome, its reason string, and the summary bucket — the single source of
// truth for the classification table in the slice spec.
func classifyOutcome(created bool, err error) (wallet.AuditOutcome, string, bucket) {
	switch {
	case err == nil && created:
		return wallet.OutcomeAccepted, "ok", bucketAccepted
	case err == nil && !created:
		return wallet.OutcomeDuplicate, "duplicate ref", bucketDuplicate
	case errors.Is(err, wallet.ErrNotFound):
		return wallet.OutcomeRejected, "account not found", bucketRejected
	case errors.Is(err, wallet.ErrInsufficientBalance):
		return wallet.OutcomeRejected, "insufficient balance", bucketRejected
	default:
		return wallet.OutcomeRejected, "rejected", bucketRejected
	}
}

// parseRow turns one CSV record into a wallet.Transaction, returning a parse
// error whose message is a STABLE reason string (it becomes the audit reason).
func parseRow(rec []string) (wallet.Transaction, error) {
	if len(rec) != len(expectedHeader) {
		return wallet.Transaction{}, errors.New("malformed row")
	}
	ref, accountID, kindStr, pointsStr, occurredStr := rec[0], rec[1], rec[2], rec[3], rec[4]

	var kind wallet.Kind
	switch kindStr {
	case "earn":
		kind = wallet.KindEarn
	case "spend":
		kind = wallet.KindSpend
	default:
		return wallet.Transaction{}, errors.New("invalid kind")
	}

	points, err := strconv.ParseInt(strings.TrimSpace(pointsStr), 10, 64)
	if err != nil || points < 1 {
		return wallet.Transaction{}, errors.New("invalid points")
	}

	occurredAt, err := time.Parse(time.RFC3339, strings.TrimSpace(occurredStr))
	if err != nil {
		return wallet.Transaction{}, errors.New("invalid occurred_at")
	}

	return wallet.Transaction{
		Ref:        ref,
		AccountID:  accountID,
		Kind:       kind,
		Points:     points,
		OccurredAt: occurredAt,
	}, nil
}

// sameHeader reports whether got matches want exactly (after trimming spaces).
func sameHeader(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range want {
		if strings.TrimSpace(got[i]) != want[i] {
			return false
		}
	}
	return true
}

// refOf / acctOf / kindOf pull the best-effort fields from a record we couldn't
// fully parse, so a rejected row's audit entry is still as faithful as possible.
func refOf(rec []string) string {
	if len(rec) > 0 {
		return rec[0]
	}
	return ""
}

func acctOf(rec []string) string {
	if len(rec) > 1 {
		return rec[1]
	}
	return ""
}

func kindOf(rec []string) string {
	if len(rec) > 2 {
		return rec[2]
	}
	return ""
}
