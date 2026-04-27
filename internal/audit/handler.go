package audit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

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
	q := r.URL.Query()
	f := Filters{}

	if v := q.Get("account_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid account_id", "INVALID_PARAM")
			return
		}
		f.AccountID = &id
	}
	if v := q.Get("outcome"); v != "" {
		o := Outcome(v)
		f.Outcome = &o
	}
	if v := q.Get("operation"); v != "" {
		f.Operation = &v
	}
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			f.Limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err == nil {
			f.Offset = n
		}
	}

	logs, err := h.store.List(r.Context(), f)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list audit log", "INTERNAL_ERROR")
		return
	}

	type response struct {
		Log
		AmountDisplay *string `json:"amount_display"`
		Currency      string  `json:"currency"`
	}
	out := make([]response, len(logs))
	for i, l := range logs {
		var display *string
		if l.Amount != nil {
			s := fmt.Sprintf("%s%.2f", h.currency.Symbol, *l.Amount)
			display = &s
		}
		out[i] = response{Log: l, AmountDisplay: display, Currency: h.currency.Code}
	}
	writeJSON(w, http.StatusOK, out)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message, code string) {
	writeJSON(w, status, map[string]string{"error": message, "code": code})
}
