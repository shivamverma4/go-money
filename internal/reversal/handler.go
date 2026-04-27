package reversal

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/shivamverma/go-money/internal/config"
	"github.com/shivamverma/go-money/internal/transaction"
)

type Handler struct {
	service  *Service
	currency config.Currency
}

func NewHandler(service *Service, currency config.Currency) *Handler {
	return &Handler{service: service, currency: currency}
}

func (h *Handler) Reverse(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid transaction id", "INVALID_ID")
		return
	}

	result, err := h.service.Reverse(r.Context(), id)
	if err != nil {
		writeError(w, HTTPStatus(err), err.Error(), ErrorCode(err))
		return
	}

	writeJSON(w, http.StatusCreated, h.toResponse(result))
}

type reversalResponse struct {
	ID             uuid.UUID          `json:"id"`
	Type           transaction.Type   `json:"type"`
	Status         transaction.Status `json:"status"`
	ReversalOfID   *uuid.UUID         `json:"reversal_of_id"`
	AmountSubunits int64              `json:"amount_subunits"`
	AmountDisplay  string             `json:"amount_display"`
	Currency       string             `json:"currency"`
}

func (h *Handler) toResponse(r Result) reversalResponse {
	var amount int64
	for _, e := range r.Entries {
		if e.DebitAmount > 0 {
			amount = e.DebitAmount
		}
	}
	return reversalResponse{
		ID:             r.Transaction.ID,
		Type:           r.Transaction.Type,
		Status:         r.Transaction.Status,
		ReversalOfID:   r.Transaction.ReversalOfID,
		AmountSubunits: amount,
		AmountDisplay:  fmt.Sprintf("%s%.2f", h.currency.Symbol, float64(amount)/100),
		Currency:       h.currency.Code,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
