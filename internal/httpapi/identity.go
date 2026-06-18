package httpapi

import "net/http"

// subjectAccountID is the identity seam. S1 has no auth yet, so the subject is
// whatever account the request names — the path param if present, else the
// caller passes the body's account_id explicitly. S3 (JWT) swaps THIS function
// to read the subject from r.Context() and enforce member-owns-account; the
// handlers that call it stay untouched. That "swap not rewrite" is the point.
//
// For the path-based routes (GET account, GET balance) the account id is a
// chi URL param; the handler hands it in via fromPath. For POST /transactions
// the id rides in the body, so the handler passes it as fromBody. Either way
// the handler reads identity ONLY through this function.
func subjectAccountID(_ *http.Request, candidate string) string {
	return candidate
}
