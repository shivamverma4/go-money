package ledger

import (
	"time"

	"github.com/google/uuid"
)

type Entry struct {
	ID            int64     `json:"id"`
	TransactionID uuid.UUID `json:"transaction_id"`
	AccountID     int64     `json:"account_id"`
	DebitAmount   int64     `json:"debit_amount"`
	CreditAmount  int64     `json:"credit_amount"`
	EntryDate     time.Time `json:"entry_date"`
}
