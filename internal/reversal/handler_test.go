package reversal_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/shivamverma/go-money/internal/config"
	"github.com/shivamverma/go-money/internal/reversal"
	"github.com/shivamverma/go-money/internal/transaction"
)

var testCurrency = config.Currency{Code: "INR", Symbol: "₹", SubunitName: "paise"}

// newReversalRouter returns a chi router wired to the reversal handler and a
// convenience transfer helper bound to the same pool.
func newReversalRouter(t *testing.T) (http.Handler, *reversal.Service) {
	t.Helper()
	pool := testDB(t)
	svc := newReversalService(pool)
	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)
	return r, svc
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestReversalHandler_InvalidUUID(t *testing.T) {
	r, _ := newReversalRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/transactions/not-a-uuid/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertReversalErrorCode(t, w, "INVALID_ID")
}

func TestReversalHandler_NotFound(t *testing.T) {
	r, _ := newReversalRouter(t)
	req := httptest.NewRequest(http.MethodPost,
		"/transactions/00000000-0000-0000-0000-000000000099/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	assertReversalErrorCode(t, w, "NOT_FOUND")
}

func TestReversalHandler_HappyPath(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000.00, 0)
	orig := doTransfer(t, pool, idA, idB, 500.00)

	svc := newReversalService(pool)
	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+orig.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d\nbody: %s", w.Code, w.Body.String())
	}
}

func TestReversalHandler_HappyPath_ResponseShape(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000.00, 0)
	orig := doTransfer(t, pool, idA, idB, 300.00)

	svc := newReversalService(pool)
	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+orig.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d\nbody: %s", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp["type"] != "reversal" {
		t.Errorf("expected type=reversal, got %v", resp["type"])
	}
	if resp["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", resp["status"])
	}
	if resp["reversal_of_id"] == nil {
		t.Errorf("expected reversal_of_id to be set")
	}
}

func TestReversalHandler_AlreadyReversed(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000.00, 0)
	orig := doTransfer(t, pool, idA, idB, 500.00)

	svc := newReversalService(pool)
	// First reversal succeeds.
	if _, err := svc.Reverse(context.Background(), orig.ID); err != nil {
		t.Fatalf("first reversal: %v", err)
	}

	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+orig.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Already reversed → ALREADY_REVERSED (409) or INVALID_STATUS (422).
	if w.Code != http.StatusConflict && w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 409 or 422, got %d\nbody: %s", w.Code, w.Body.String())
	}
}

func TestReversalHandler_ReversalOfReversal(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000.00, 0)
	orig := doTransfer(t, pool, idA, idB, 500.00)

	svc := newReversalService(pool)
	rev, err := svc.Reverse(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("reversal: %v", err)
	}

	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+rev.Transaction.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d\nbody: %s", w.Code, w.Body.String())
	}
	assertReversalErrorCode(t, w, "INVALID_TYPE")
}

func TestReversalHandler_FailedTransaction(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1.00, 0)

	// This transfer will fail (insufficient funds).
	result, _ := newTxService(pool).Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID: idA,
		ToAccountID:   idB,
		Amount:        99.99,
	})

	svc := newReversalService(pool)
	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+result.Transaction.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d\nbody: %s", w.Code, w.Body.String())
	}
	assertReversalErrorCode(t, w, "INVALID_STATUS")
}

func TestReversalHandler_InsufficientFundsAtDestination(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000.00, 0)
	orig := doTransfer(t, pool, idA, idB, 1000.00)

	// Drain B so the reversal cannot pull funds back.
	doTransfer(t, pool, idB, idA, 1000.00)

	svc := newReversalService(pool)
	h := reversal.NewHandler(svc, testCurrency)
	r := chi.NewRouter()
	r.Post("/transactions/{id}/reverse", h.Reverse)

	req := httptest.NewRequest(http.MethodPost,
		"/transactions/"+orig.ID.String()+"/reverse", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d\nbody: %s", w.Code, w.Body.String())
	}
	assertReversalErrorCode(t, w, "INSUFFICIENT_FUNDS_FOR_REVERSAL")
}

// ── Helpers ────────────────────────────────────────────────────────────────

func assertReversalErrorCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse error response: %v\nbody: %s", err, w.Body.String())
	}
	if body["code"] != want {
		t.Errorf("expected error code %q, got %q", want, body["code"])
	}
}
