package account

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/shivamverma/go-money/internal/config"
)

type Handler struct {
	store    *Store
	currency config.Currency
}

func NewHandler(store *Store, currency config.Currency) *Handler {
	return &Handler{store: store, currency: currency}
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.store.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list accounts", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, h.toResponses(accounts))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid account id", "INVALID_ID")
		return
	}
	a, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "account not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get account", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, h.toResponse(a))
}

func (h *Handler) ListByCustomer(w http.ResponseWriter, r *http.Request) {
	customerID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid customer id", "INVALID_ID")
		return
	}
	accounts, err := h.store.ListByCustomer(r.Context(), customerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list accounts", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, h.toResponses(accounts))
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CustomerID     int64   `json:"customer_id"`
		Currency       string  `json:"currency"`
		InitialBalance float64 `json:"initial_balance"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}
	if req.CustomerID == 0 {
		writeError(w, http.StatusBadRequest, "customer_id is required", "MISSING_FIELDS")
		return
	}
	if req.Currency == "" {
		req.Currency = h.currency.Code
	}
	if req.InitialBalance < 0 {
		writeError(w, http.StatusBadRequest, "initial_balance must be >= 0", "INVALID_AMOUNT")
		return
	}
	a, err := h.store.Create(r.Context(), req.CustomerID, req.Currency, req.InitialBalance)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create account", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusCreated, h.toResponse(a))
}

// Response shape returned to callers.
type Response struct {
	ID             int64   `json:"id"`
	CustomerID     int64   `json:"customer_id"`
	Currency       string  `json:"currency"`
	Balance        float64 `json:"balance"`
	BalanceDisplay string  `json:"balance_display"`
	Status         Status  `json:"status"`
}

func (h *Handler) toResponse(a Account) Response {
	return Response{
		ID:             a.ID,
		CustomerID:     a.CustomerID,
		Currency:       a.Currency,
		Balance:        a.Balance,
		BalanceDisplay: formatAmount(a.Balance, h.currency.Symbol),
		Status:         a.Status,
	}
}

func (h *Handler) toResponses(accounts []Account) []Response {
	out := make([]Response, len(accounts))
	for i, a := range accounts {
		out[i] = h.toResponse(a)
	}
	return out
}

// formatAmount formats a rupee value with Indian comma notation, e.g. 1000.50 → "₹1,000.50"
func formatAmount(rupees float64, symbol string) string {
	whole := int64(rupees)
	frac := int64(math.Round((rupees-float64(whole))*100))
	return fmt.Sprintf("%s%s.%02d", symbol, formatWithCommas(whole), frac)
}

func formatWithCommas(n int64) string {
	s := strconv.FormatInt(n, 10)
	// Indian numbering: last 3 digits, then groups of 2
	if len(s) <= 3 {
		return s
	}
	result := s[len(s)-3:]
	s = s[:len(s)-3]
	for len(s) > 2 {
		result = s[len(s)-2:] + "," + result
		s = s[:len(s)-2]
	}
	if len(s) > 0 {
		result = s + "," + result
	}
	return result
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
