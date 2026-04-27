// Package types holds shared domain types imported by multiple packages
// to avoid circular dependencies.
package types

type TransactionType string

const (
	TransactionTypeTransfer   TransactionType = "transfer"
	TransactionTypeReversal   TransactionType = "reversal"
	TransactionTypeDeposit    TransactionType = "deposit"
	TransactionTypeWithdrawal TransactionType = "withdrawal"
)

type TransactionStatus string

const (
	TransactionStatusCompleted TransactionStatus = "completed"
	TransactionStatusFailed    TransactionStatus = "failed"
	TransactionStatusReversed  TransactionStatus = "reversed"
)

type AuditOutcome string

const (
	AuditOutcomeSuccess AuditOutcome = "success"
	AuditOutcomeFailure AuditOutcome = "failure"
)
