package audit

import (
	"time"

	"github.com/google/uuid"

	"github.com/shivamverma/go-money/internal/types"
)

type Outcome = types.AuditOutcome

const (
	OutcomeSuccess = types.AuditOutcomeSuccess
	OutcomeFailure = types.AuditOutcomeFailure
)

type Log struct {
	ID            int64                 `json:"id"`
	Operation     types.TransactionType `json:"operation"`
	TransactionID *uuid.UUID            `json:"transaction_id"`
	AccountIDs    []int64               `json:"account_ids"`
	Amount        *float64              `json:"amount"`
	Outcome       Outcome               `json:"outcome"`
	FailureReason *string               `json:"failure_reason"`
	CreatedAt     time.Time             `json:"created_at"`
}
