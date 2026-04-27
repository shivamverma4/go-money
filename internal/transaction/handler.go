package transaction

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/shivamverma/go-money/internal/config"
	"github.com/shivamverma/go-money/internal/ledger"
)

type Handler struct {
	service  *Service
	store    *Store
	currency config.Currency
}

func NewHandler(service *Service, store *Store, currency config.Currency) *Handler {
	return &Handler{service: service, store: store, currency: currency}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FromAccountID  int64   `json:"from_account_id"`
		ToAccountID    int64   `json:"to_account_id"`
		AmountSubunits int64   `json:"amount_subunits"`
		ReferenceID    *string `json:"reference_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body", "INVALID_BODY")
		return
	}

	result, err := h.service.Transfer(r.Context(), TransferRequest{
		FromAccountID:  req.FromAccountID,
		ToAccountID:    req.ToAccountID,
		AmountSubunits: req.AmountSubunits,
		ReferenceID:    req.ReferenceID,
	})

	if err != nil {
		var ae *AppError
		if errors.As(err, &ae) {
			writeError(w, http.StatusUnprocessableEntity, ae.Message, ae.Code)
			return
		}
		writeError(w, http.StatusInternalServerError, "transfer failed", "INTERNAL_ERROR")
		return
	}

	writeJSON(w, http.StatusCreated, h.toResponse(result.Transaction, result.Entries))
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id", "INVALID_ID")
		return
	}
	tx, err := h.store.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "transaction not found", "NOT_FOUND")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to get transaction", "INTERNAL_ERROR")
		return
	}
	entries, err := h.service.GetLedgerEntries(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get ledger entries", "INTERNAL_ERROR")
		return
	}
	writeJSON(w, http.StatusOK, h.toResponse(tx, entries))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}

	txs, err := h.store.List(r.Context(), limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list transactions", "INTERNAL_ERROR")
		return
	}
	out := make([]transactionResponse, len(txs))
	for i, tx := range txs {
		out[i] = h.toResponse(tx, nil)
	}
	writeJSON(w, http.StatusOK, out)
}

type transactionResponse struct {
	ID             uuid.UUID      `json:"id"`
	Type           Type           `json:"type"`
	Status         Status         `json:"status"`
	ReferenceID    *string        `json:"reference_id"`
	ReversalOfID   *uuid.UUID     `json:"reversal_of_id"`
	FromAccountID  *int64         `json:"from_account_id"`
	ToAccountID    *int64         `json:"to_account_id"`
	AmountSubunits *int64         `json:"amount_subunits"`
	AmountDisplay  *string        `json:"amount_display"`
	Currency       *string        `json:"currency"`
	FailureReason  *string        `json:"failure_reason"`
	LedgerEntries  []ledger.Entry `json:"ledger_entries,omitempty"`
}

func (h *Handler) toResponse(tx Transaction, entries []ledger.Entry) transactionResponse {
	resp := transactionResponse{
		ID:            tx.ID,
		Type:          tx.Type,
		Status:        tx.Status,
		ReferenceID:   tx.ReferenceID,
		ReversalOfID:  tx.ReversalOfID,
		FailureReason: tx.FailureReason,
		LedgerEntries: entries,
	}

	// Derive from/to/amount from ledger entries when available.
	for _, e := range entries {
		if e.DebitAmount > 0 {
			id := e.AccountID
			resp.FromAccountID = &id
			resp.AmountSubunits = &e.DebitAmount
			display := formatAmount(e.DebitAmount, h.currency.Symbol)
			resp.AmountDisplay = &display
			cur := h.currency.Code
			resp.Currency = &cur
		}
		if e.CreditAmount > 0 {
			id := e.AccountID
			resp.ToAccountID = &id
		}
	}

	return resp
}

func formatAmount(subunits int64, symbol string) string {
	return fmt.Sprintf("%s%.2f", symbol, float64(subunits)/100)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
