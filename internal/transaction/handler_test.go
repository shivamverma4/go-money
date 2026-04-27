package transaction_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/audit"
	"github.com/shivamverma/go-money/internal/config"
	"github.com/shivamverma/go-money/internal/ledger"
	"github.com/shivamverma/go-money/internal/transaction"
)

var testCurrency = config.Currency{Code: "INR", Symbol: "₹", SubunitName: "paise"}

// newTestRouter wires up the transaction handler routes against a real test DB.
func newTestRouter(t *testing.T) (http.Handler, int64, int64) {
	t.Helper()
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 500000, 0)

	svc := transaction.NewService(
		pool,
		account.NewStore(pool),
		transaction.NewStore(pool),
		ledger.NewStore(),
		audit.NewStore(pool),
	)
	store := transaction.NewStore(pool)
	h := transaction.NewHandler(svc, store, testCurrency)

	r := chi.NewRouter()
	r.Get("/transactions", h.List)
	r.Post("/transactions", h.Create)
	r.Get("/transactions/{id}", h.Get)

	return r, idA, idB
}

// ── List ───────────────────────────────────────────────────────────────────

func TestTransactionHandler_List_ReturnsJSON(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/transactions", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestTransactionHandler_List_DefaultsToEmptyArray(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/transactions?limit=10&offset=0", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result []json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("response is not a JSON array: %v", err)
	}
}

// ── Create ─────────────────────────────────────────────────────────────────

func TestTransactionHandler_Create_InvalidJSON(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/transactions",
		bytes.NewBufferString(`{not json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "INVALID_BODY")
}

func TestTransactionHandler_Create_Success(t *testing.T) {
	r, idA, idB := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{
		"from_account_id": idA,
		"to_account_id":   idB,
		"amount_subunits": 10000,
	})
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp["status"] != "completed" {
		t.Errorf("expected status completed, got %v", resp["status"])
	}
}

func TestTransactionHandler_Create_InsufficientFunds(t *testing.T) {
	r, idA, idB := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{
		"from_account_id": idA,
		"to_account_id":   idB,
		"amount_subunits": 999999999, // way more than balance
	})
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", w.Code)
	}
	assertErrorCode(t, w, "INSUFFICIENT_FUNDS")
}

func TestTransactionHandler_Create_SameAccount(t *testing.T) {
	r, idA, _ := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{
		"from_account_id": idA,
		"to_account_id":   idA,
		"amount_subunits": 100,
	})
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for same-account transfer, got %d", w.Code)
	}
	assertErrorCode(t, w, "SAME_ACCOUNT")
}

func TestTransactionHandler_Create_ZeroAmount(t *testing.T) {
	r, idA, idB := newTestRouter(t)
	body, _ := json.Marshal(map[string]any{
		"from_account_id": idA,
		"to_account_id":   idB,
		"amount_subunits": 0,
	})
	req := httptest.NewRequest(http.MethodPost, "/transactions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for zero amount, got %d", w.Code)
	}
	assertErrorCode(t, w, "INVALID_AMOUNT")
}

// ── Get ────────────────────────────────────────────────────────────────────

func TestTransactionHandler_Get_InvalidUUID(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/transactions/not-a-uuid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "INVALID_ID")
}

func TestTransactionHandler_Get_NotFound(t *testing.T) {
	r, _, _ := newTestRouter(t)
	req := httptest.NewRequest(http.MethodGet,
		"/transactions/00000000-0000-0000-0000-000000000099", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestTransactionHandler_Get_Success(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)
	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 5000,
	})
	if err != nil {
		t.Fatalf("setup transfer: %v", err)
	}

	store := transaction.NewStore(pool)
	h := transaction.NewHandler(svc, store, testCurrency)
	r := chi.NewRouter()
	r.Get("/transactions/{id}", h.Get)

	req := httptest.NewRequest(http.MethodGet,
		fmt.Sprintf("/transactions/%s", result.Transaction.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp["id"] != result.Transaction.ID.String() {
		t.Errorf("ID mismatch: got %v", resp["id"])
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse error response: %v\nbody: %s", err, w.Body.String())
	}
	if body["code"] != want {
		t.Errorf("expected error code %q, got %q", want, body["code"])
	}
}
