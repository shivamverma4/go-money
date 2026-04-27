package reversal

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/audit"
	"github.com/shivamverma/go-money/internal/ledger"
	"github.com/shivamverma/go-money/internal/transaction"
)

type Result struct {
	Transaction transaction.Transaction
	Entries     []ledger.Entry
}

type Service struct {
	db           *pgxpool.Pool
	accountStore *account.Store
	txStore      *transaction.Store
	ledgerStore  *ledger.Store
	auditStore   *audit.Store
}

func NewService(
	db *pgxpool.Pool,
	accountStore *account.Store,
	txStore *transaction.Store,
	ledgerStore *ledger.Store,
	auditStore *audit.Store,
) *Service {
	return &Service{
		db:           db,
		accountStore: accountStore,
		txStore:      txStore,
		ledgerStore:  ledgerStore,
		auditStore:   auditStore,
	}
}

func (s *Service) Reverse(ctx context.Context, originalID uuid.UUID) (Result, error) {
	var result Result

	err := pgx.BeginTxFunc(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		orig, err := s.txStore.GetByIDForUpdate(ctx, tx, originalID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &appError{"transaction not found", "NOT_FOUND", http404}
			}
			return err
		}
		if orig.Type != transaction.TypeTransfer {
			return &appError{"only transfer transactions can be reversed", "INVALID_TYPE", http422}
		}
		if orig.Status != transaction.StatusCompleted {
			return &appError{
				fmt.Sprintf("cannot reverse a transaction with status '%s'", orig.Status),
				"INVALID_STATUS", http422,
			}
		}

		entries, err := s.ledgerStore.ListByTransaction(ctx, tx, originalID)
		if err != nil {
			return fmt.Errorf("fetch original entries: %w", err)
		}
		var fromID, toID int64
		var amount float64
		for _, e := range entries {
			if e.DebitAmount > 0 {
				fromID = e.AccountID
				amount = e.DebitAmount
			}
			if e.CreditAmount > 0 {
				toID = e.AccountID
			}
		}
		if amount == 0 {
			return fmt.Errorf("original transaction has no ledger entries")
		}

		firstID, secondID := fromID, toID
		if firstID > secondID {
			firstID, secondID = secondID, firstID
		}
		first, err := s.accountStore.GetByIDForUpdate(ctx, tx, firstID)
		if err != nil {
			return fmt.Errorf("lock account %d: %w", firstID, err)
		}
		second, err := s.accountStore.GetByIDForUpdate(ctx, tx, secondID)
		if err != nil {
			return fmt.Errorf("lock account %d: %w", secondID, err)
		}

		var origFrom, origTo account.Account
		if firstID == fromID {
			origFrom, origTo = first, second
		} else {
			origFrom, origTo = second, first
		}

		if origTo.Balance-amount < origTo.Floor {
			return &appError{"destination account has insufficient funds for reversal", "INSUFFICIENT_FUNDS_FOR_REVERSAL", http422}
		}

		if err := s.accountStore.UpdateBalance(ctx, tx, origFrom.ID, amount); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, origTo.ID, -amount); err != nil {
			return err
		}

		reversalOfID := originalID
		reversalTx, err := s.txStore.Create(ctx, tx, transaction.Transaction{
			Type:         transaction.TypeReversal,
			Status:       transaction.StatusCompleted,
			ReversalOfID: &reversalOfID,
		})
		if err != nil {
			return fmt.Errorf("create reversal transaction: %w", err)
		}

		debitEntry := ledger.Entry{TransactionID: reversalTx.ID, AccountID: origTo.ID, DebitAmount: amount}
		creditEntry := ledger.Entry{TransactionID: reversalTx.ID, AccountID: origFrom.ID, CreditAmount: amount}
		if err := s.ledgerStore.InsertEntry(ctx, tx, debitEntry); err != nil {
			return err
		}
		if err := s.ledgerStore.InsertEntry(ctx, tx, creditEntry); err != nil {
			return err
		}

		if err := s.txStore.MarkReversed(ctx, tx, originalID); err != nil {
			return err
		}

		reversalTxID := reversalTx.ID
		if err := s.auditStore.Insert(ctx, tx, audit.Log{
			Operation:     transaction.TypeReversal,
			TransactionID: &reversalTxID,
			AccountIDs:    []int64{origFrom.ID, origTo.ID},
			Amount:        &amount,
			Outcome:       audit.OutcomeSuccess,
		}); err != nil {
			return err
		}

		result = Result{Transaction: reversalTx, Entries: []ledger.Entry{debitEntry, creditEntry}}
		return nil
	})

	if err != nil {
		var ae *appError
		if errors.As(err, &ae) {
			return Result{}, ae
		}
		if isUniqueViolation(err) {
			return Result{}, &appError{"transaction has already been reversed", "ALREADY_REVERSED", http409}
		}
		return Result{}, fmt.Errorf("reverse: %w", err)
	}

	return result, nil
}

func HTTPStatus(err error) int {
	var ae *appError
	if errors.As(err, &ae) {
		return ae.status
	}
	return 500
}

func ErrorCode(err error) string {
	var ae *appError
	if errors.As(err, &ae) {
		return ae.code
	}
	return "INTERNAL_ERROR"
}

const (
	http404 = 404
	http409 = 409
	http422 = 422
)

type appError struct {
	msg    string
	code   string
	status int
}

func (e *appError) Error() string { return e.msg }

func isUniqueViolation(err error) bool {
	return err != nil && len(err.Error()) > 0 &&
		(contains(err.Error(), "unique") || contains(err.Error(), "23505"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
