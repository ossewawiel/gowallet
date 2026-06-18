package acceptance_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// batchSummary mirrors the BatchSummary schema on the wire.
type batchSummary struct {
	Processed  int64 `json:"processed"`
	Accepted   int64 `json:"accepted"`
	Rejected   int64 `json:"rejected"`
	Duplicates int64 `json:"duplicates"`
}

const csvHeader = "ref,account_id,kind,points,occurred_at\n"

// postBatch uploads csv as a multipart `file` part to POST /batch with the
// given bearer token, returning the raw response.
func postBatch(t *testing.T, base, token, csv string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("file", "batch.csv")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(csv)); err != nil {
		t.Fatalf("write csv: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/batch", &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /batch: %v", err)
	}
	return resp
}

// uploadBatch posts a batch as admin, asserts 200, and decodes the summary.
func uploadBatch(t *testing.T, base, admin, csv string) batchSummary {
	t.Helper()
	resp := postBatch(t, base, admin, csv)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST /batch: want 200, got %d", resp.StatusCode)
	}
	var sum batchSummary
	if err := json.NewDecoder(resp.Body).Decode(&sum); err != nil {
		t.Fatalf("decode summary: %v", err)
	}
	return sum
}

// row builds one CSV data line.
func row(ref, id, kind string, pts int64) string {
	return fmt.Sprintf("%s,%s,%s,%d,2024-06-01T10:00:00Z\n", ref, id, kind, pts)
}

// TestBatch_Reprocess_Idempotent (INV-9) — uploading the identical file twice
// applies every row once; the second pass is all duplicates, balances unchanged.
func TestBatch_Reprocess_Idempotent(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")
	createAccount(t, srv.URL, "member-2")

	csv := csvHeader +
		row("e1", "member-1", "earn", 100) +
		row("e2", "member-2", "earn", 50) +
		row("s1", "member-1", "spend", 30)

	first := uploadBatch(t, srv.URL, admin, csv)
	if first.Processed != 3 || first.Accepted != 3 || first.Duplicates != 0 || first.Rejected != 0 {
		t.Fatalf("first pass: want 3/3/0/0, got %+v", first)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 70 {
		t.Fatalf("member-1 after first pass: want 70, got %d", bal)
	}

	second := uploadBatch(t, srv.URL, admin, csv)
	if second.Processed != 3 || second.Accepted != 0 || second.Duplicates != 3 || second.Rejected != 0 {
		t.Fatalf("second pass: want 3/0/3/0, got %+v", second)
	}
	if bal := getBalance(t, srv.URL, "member-1"); bal != 70 {
		t.Fatalf("member-1 after reprocess: want 70 (unchanged), got %d", bal)
	}
	if bal := getBalance(t, srv.URL, "member-2"); bal != 50 {
		t.Fatalf("member-2 after reprocess: want 50 (unchanged), got %d", bal)
	}
}

// TestBatch_Summary (INV-10) — a mixed file yields exact counts and
// processed == accepted + rejected + duplicates.
func TestBatch_Summary(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")

	// Seed a duplicate ref via a first single-row file.
	uploadBatch(t, srv.URL, admin, csvHeader+row("dup", "member-1", "earn", 10))

	csv := csvHeader +
		row("fresh-1", "member-1", "earn", 100) + // accepted
		row("fresh-2", "member-1", "earn", 50) + // accepted
		row("fresh-3", "member-1", "spend", 20) + // accepted
		row("dup", "member-1", "earn", 10) + // duplicate
		row("over", "member-1", "spend", 99999) + // rejected: insufficient
		row("ghost", "no-such-acct", "earn", 10) + // rejected: account not found
		"bad,row\n" // rejected: malformed

	sum := uploadBatch(t, srv.URL, admin, csv)
	if sum.Processed != 7 {
		t.Fatalf("processed: want 7, got %d", sum.Processed)
	}
	if sum.Accepted != 3 || sum.Duplicates != 1 || sum.Rejected != 3 {
		t.Fatalf("summary: want accepted=3 duplicates=1 rejected=3, got %+v", sum)
	}
	if sum.Processed != sum.Accepted+sum.Rejected+sum.Duplicates {
		t.Fatalf("invariant violated: processed=%d != %d", sum.Processed, sum.Accepted+sum.Rejected+sum.Duplicates)
	}
}

// TestBatch_AuditsEachRow (INV-23) — every data row produces exactly one audit
// entry with the expected outcome + non-empty reason.
func TestBatch_AuditsEachRow(t *testing.T) {
	srv, _ := bootRealAppWithStore(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")

	csv := csvHeader +
		row("a1", "member-1", "earn", 100) + // accepted
		row("a2", "member-1", "spend", 999) + // rejected
		"junk\n" // rejected malformed

	sum := uploadBatch(t, srv.URL, admin, csv)
	if sum.Processed != 3 {
		t.Fatalf("processed: want 3, got %d", sum.Processed)
	}

	entries := getAudit(t, srv.URL, "", admin)
	if len(entries) != 3 {
		t.Fatalf("audit rows after batch: want 3, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Outcome == "" || e.Reason == "" {
			t.Fatalf("audit row %q: want non-empty outcome+reason, got %+v", e.Ref, e)
		}
	}
}

// TestBatch_SameAccountCloseTogether (trap) — many earns+spends for one account
// in a single file → final balance is exactly Σearn − Σspend.
func TestBatch_SameAccountCloseTogether(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")

	var b strings.Builder
	b.WriteString(csvHeader)
	var wantEarn, wantSpend int64
	for i := 0; i < 20; i++ {
		b.WriteString(row(fmt.Sprintf("e%d", i), "member-1", "earn", 100))
		wantEarn += 100
	}
	for i := 0; i < 20; i++ {
		b.WriteString(row(fmt.Sprintf("s%d", i), "member-1", "spend", 30))
		wantSpend += 30
	}

	sum := uploadBatch(t, srv.URL, admin, b.String())
	if sum.Accepted != 40 || sum.Rejected != 0 {
		t.Fatalf("same-account file: want 40 accepted / 0 rejected, got %+v", sum)
	}
	want := wantEarn - wantSpend
	if bal := getBalance(t, srv.URL, "member-1"); bal != want {
		t.Fatalf("final balance: want %d, got %d", want, bal)
	}
}

// TestBatch_ConcurrentReprocess_Idempotent (-race, INV-9 under contention) —
// the same file uploaded twice concurrently: each ref applied exactly once.
func TestBatch_ConcurrentReprocess_Idempotent(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")

	csv := csvHeader +
		row("c1", "member-1", "earn", 100) +
		row("c2", "member-1", "earn", 200) +
		row("c3", "member-1", "spend", 50)

	const n = 4
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			resp := postBatch(t, srv.URL, admin, csv)
			_ = resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("concurrent batch: want 200, got %d", resp.StatusCode)
			}
		}()
	}
	wg.Wait()

	// Net = 100 + 200 - 50 = 250, applied exactly once regardless of concurrency.
	if bal := getBalance(t, srv.URL, "member-1"); bal != 250 {
		t.Fatalf("balance after concurrent reprocess: want 250, got %d", bal)
	}
}

// TestBatch_AdminOnly — member → 403, admin → 200.
func TestBatch_AdminOnly(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)
	createAccount(t, srv.URL, "member-1")

	memberTok := mintToken(t, srv.URL, "member-1", "member")
	resp := postBatch(t, srv.URL, memberTok, csvHeader+row("m1", "member-1", "earn", 10))
	memberStatus := resp.StatusCode
	_ = resp.Body.Close()
	if memberStatus != http.StatusForbidden {
		t.Fatalf("member POST /batch: want 403, got %d", memberStatus)
	}

	adminResp := postBatch(t, srv.URL, admin, csvHeader+row("a1", "member-1", "earn", 10))
	adminStatus := adminResp.StatusCode
	_ = adminResp.Body.Close()
	if adminStatus != http.StatusOK {
		t.Fatalf("admin POST /batch: want 200, got %d", adminStatus)
	}
}

// TestBatch_NoToken_401 — no token → 401.
func TestBatch_NoToken_401(t *testing.T) {
	srv := bootRealApp(t)
	resp := postBatch(t, srv.URL, "", csvHeader+row("x", "member-1", "earn", 10))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token POST /batch: want 401, got %d", resp.StatusCode)
	}
}

// TestBatch_BadUpload_400 — missing file part / unrecognised header → 400 with
// the standard error envelope.
func TestBatch_BadUpload_400(t *testing.T) {
	srv := bootRealApp(t)
	admin := adminToken(t, srv.URL)

	// No `file` part at all (other field) → 400.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("nope", "x")
	_ = mw.Close()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/batch", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+admin)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST /batch: %v", err)
	}
	missingStatus := resp.StatusCode
	_ = resp.Body.Close()
	if missingStatus != http.StatusBadRequest {
		t.Fatalf("missing file part: want 400, got %d", missingStatus)
	}

	// File present but no recognisable CSV header → 400.
	resp2 := postBatch(t, srv.URL, admin, "wrong,columns,here\nvalue,value,value\n")
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("unrecognised header: want 400, got %d", resp2.StatusCode)
	}
	var env struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&env); err != nil {
		t.Fatalf("decode error envelope: %v", err)
	}
	if env.Error.Code == "" {
		t.Fatalf("bad upload: want error envelope with code, got %+v", env)
	}
}
