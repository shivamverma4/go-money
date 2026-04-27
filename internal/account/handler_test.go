package account_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/config"
)

var testCurrency = config.Currency{Code: "INR", Symbol: "₹", SubunitName: "paise"}

func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:password@localhost:5432/go_money_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedCustomer inserts a customer and returns its ID. Cleans up on test exit.
func seedCustomer(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var custID int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO customers (name, email) VALUES ('Test', $1) RETURNING id`,
		"acct-test-"+t.Name()+"@example.com",
	).Scan(&custID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM accounts WHERE customer_id = $1`, custID)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, custID)
	})
	return custID
}

func newAccountRouter(t *testing.T) (http.Handler, *pgxpool.Pool) {
	t.Helper()
	pool := testDB(t)
	store := account.NewStore(pool)
	h := account.NewHandler(store, testCurrency)

	r := chi.NewRouter()
	r.Get("/accounts", h.List)
	r.Post("/accounts", h.Create)
	r.Get("/accounts/{id}", h.Get)
	r.Get("/customers/{id}/accounts", h.ListByCustomer)

	return r, pool
}

// ── List ───────────────────────────────────────────────────────────────────

func TestAccountHandler_List_ReturnsJSON(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
	}
}

func TestAccountHandler_List_ReturnsArray(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/accounts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var result []json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Errorf("response is not a JSON array: %v\nbody: %s", err, w.Body.String())
	}
}

// ── Get ────────────────────────────────────────────────────────────────────

func TestAccountHandler_Get_InvalidID(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/accounts/not-a-number", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "INVALID_ID")
}

func TestAccountHandler_Get_NotFound(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/accounts/999999999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "NOT_FOUND")
}

func TestAccountHandler_Get_Success(t *testing.T) {
	r, pool := newAccountRouter(t)
	custID := seedCustomer(t, pool)

	store := account.NewStore(pool)
	a, err := store.Create(context.Background(), custID, "INR", 50000)
	if err != nil {
		t.Fatalf("create account: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet,
		"/accounts/"+itoa(a.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if int64(resp["id"].(float64)) != a.ID {
		t.Errorf("ID mismatch: got %v, want %d", resp["id"], a.ID)
	}
}

// ── Create ─────────────────────────────────────────────────────────────────

func TestAccountHandler_Create_InvalidJSON(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/accounts",
		bytes.NewBufferString(`{not json}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "INVALID_BODY")
}

func TestAccountHandler_Create_MissingCustomerID(t *testing.T) {
	r, _ := newAccountRouter(t)
	body, _ := json.Marshal(map[string]any{
		"currency":               "INR",
		"initial_balance_subunits": 1000,
	})
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "MISSING_FIELDS")
}

func TestAccountHandler_Create_NegativeBalance(t *testing.T) {
	r, pool := newAccountRouter(t)
	custID := seedCustomer(t, pool)

	body, _ := json.Marshal(map[string]any{
		"customer_id":              custID,
		"currency":                "INR",
		"initial_balance_subunits": -100,
	})
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "INVALID_AMOUNT")
}

func TestAccountHandler_Create_Success(t *testing.T) {
	r, pool := newAccountRouter(t)
	custID := seedCustomer(t, pool)

	body, _ := json.Marshal(map[string]any{
		"customer_id":              custID,
		"currency":                "INR",
		"initial_balance_subunits": 100000,
	})
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
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
	if resp["status"] != "active" {
		t.Errorf("expected status=active, got %v", resp["status"])
	}
	if int64(resp["customer_id"].(float64)) != custID {
		t.Errorf("customer_id mismatch: got %v, want %d", resp["customer_id"], custID)
	}
}

func TestAccountHandler_Create_DefaultsCurrency(t *testing.T) {
	r, pool := newAccountRouter(t)
	custID := seedCustomer(t, pool)

	// Omit currency — should default to testCurrency.Code ("INR").
	body, _ := json.Marshal(map[string]any{
		"customer_id": custID,
	})
	req := httptest.NewRequest(http.MethodPost, "/accounts", bytes.NewBuffer(body))
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
	if resp["currency"] != "INR" {
		t.Errorf("expected currency=INR, got %v", resp["currency"])
	}
}

// ── ListByCustomer ─────────────────────────────────────────────────────────

func TestAccountHandler_ListByCustomer_InvalidID(t *testing.T) {
	r, _ := newAccountRouter(t)
	req := httptest.NewRequest(http.MethodGet, "/customers/abc/accounts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
	assertAccountErrorCode(t, w, "INVALID_ID")
}

func TestAccountHandler_ListByCustomer_Success(t *testing.T) {
	r, pool := newAccountRouter(t)
	custID := seedCustomer(t, pool)

	store := account.NewStore(pool)
	store.Create(context.Background(), custID, "INR", 10000)
	store.Create(context.Background(), custID, "INR", 20000)

	req := httptest.NewRequest(http.MethodGet,
		"/customers/"+itoa(custID)+"/accounts", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d\nbody: %s", w.Code, w.Body.String())
	}
	var result []json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(result))
	}
}

// ── Helpers ────────────────────────────────────────────────────────────────

func assertAccountErrorCode(t *testing.T, w *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("parse error response: %v\nbody: %s", err, w.Body.String())
	}
	if body["code"] != want {
		t.Errorf("expected error code %q, got %q", want, body["code"])
	}
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := make([]byte, 0, 20)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	if neg {
		return "-" + string(buf)
	}
	return string(buf)
}
