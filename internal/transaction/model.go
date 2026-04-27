package transaction

import (
	"time"

	"github.com/google/uuid"

	"github.com/shivamverma/go-money/internal/types"
)

// Re-export types for convenience so callers don't need two imports.
type Type = types.TransactionType
type Status = types.TransactionStatus

const (
	TypeTransfer   = types.TransactionTypeTransfer
	TypeReversal   = types.TransactionTypeReversal
	TypeDeposit    = types.TransactionTypeDeposit
	TypeWithdrawal = types.TransactionTypeWithdrawal

	StatusCompleted = types.TransactionStatusCompleted
	StatusFailed    = types.TransactionStatusFailed
	StatusReversed  = types.TransactionStatusReversed
)

type Transaction struct {
	ID            uuid.UUID `json:"id"`
	Type          Type      `json:"type"`
	Status        Status    `json:"status"`
	ReferenceID   *string   `json:"reference_id"`
	ReversalOfID  *uuid.UUID `json:"reversal_of_id"`
	FailureReason *string   `json:"failure_reason"`
	CreatedAt     time.Time `json:"created_at"`
}
