package transaction

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
)

type TransferRequest struct {
	FromAccountID  int64
	ToAccountID    int64
	AmountSubunits int64
	ReferenceID    *string
}

type TransferResult struct {
	Transaction Transaction
	Entries     []ledger.Entry
}

type Service struct {
	db           *pgxpool.Pool
	accountStore *account.Store
	txStore      *Store
	ledgerStore  *ledger.Store
	auditStore   *audit.Store
}

func NewService(
	db *pgxpool.Pool,
	accountStore *account.Store,
	txStore *Store,
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

func (s *Service) Transfer(ctx context.Context, req TransferRequest) (TransferResult, error) {
	if req.AmountSubunits <= 0 {
		return s.recordFailure(ctx, req, "amount must be greater than zero", "INVALID_AMOUNT")
	}
	if req.FromAccountID == req.ToAccountID {
		return s.recordFailure(ctx, req, "source and destination accounts must differ", "SAME_ACCOUNT")
	}

	var result TransferResult

	err := pgx.BeginTxFunc(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		firstID, secondID := req.FromAccountID, req.ToAccountID
		if firstID > secondID {
			firstID, secondID = secondID, firstID
		}
		first, err := s.accountStore.GetByIDForUpdate(ctx, tx, firstID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &validationError{msg: fmt.Sprintf("account %d not found", firstID), code: "ACCOUNT_NOT_FOUND"}
			}
			return err
		}
		second, err := s.accountStore.GetByIDForUpdate(ctx, tx, secondID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return &validationError{msg: fmt.Sprintf("account %d not found", secondID), code: "ACCOUNT_NOT_FOUND"}
			}
			return err
		}

		var fromAcc, toAcc account.Account
		if firstID == req.FromAccountID {
			fromAcc, toAcc = first, second
		} else {
			fromAcc, toAcc = second, first
		}

		if fromAcc.Status != account.StatusActive {
			return &validationError{msg: "source account is not active", code: "ACCOUNT_NOT_ACTIVE"}
		}
		if toAcc.Status != account.StatusActive {
			return &validationError{msg: "destination account is not active", code: "ACCOUNT_NOT_ACTIVE"}
		}
		if fromAcc.Currency != toAcc.Currency {
			return &validationError{msg: "accounts have different currencies", code: "CURRENCY_MISMATCH"}
		}
		if fromAcc.Balance-req.AmountSubunits < fromAcc.Floor {
			return &validationError{msg: "insufficient funds", code: "INSUFFICIENT_FUNDS"}
		}

		if err := s.accountStore.UpdateBalance(ctx, tx, req.FromAccountID, -req.AmountSubunits); err != nil {
			return err
		}
		if err := s.accountStore.UpdateBalance(ctx, tx, req.ToAccountID, req.AmountSubunits); err != nil {
			return err
		}

		created, err := s.txStore.Create(ctx, tx, Transaction{
			Type:        TypeTransfer,
			Status:      StatusCompleted,
			ReferenceID: req.ReferenceID,
		})
		if err != nil {
			return err
		}

		debitEntry := ledger.Entry{TransactionID: created.ID, AccountID: req.FromAccountID, DebitAmount: req.AmountSubunits}
		creditEntry := ledger.Entry{TransactionID: created.ID, AccountID: req.ToAccountID, CreditAmount: req.AmountSubunits}
		if err := s.ledgerStore.InsertEntry(ctx, tx, debitEntry); err != nil {
			return err
		}
		if err := s.ledgerStore.InsertEntry(ctx, tx, creditEntry); err != nil {
			return err
		}

		txID := created.ID
		amt := req.AmountSubunits
		if err := s.auditStore.Insert(ctx, tx, audit.Log{
			Operation:      TypeTransfer,
			TransactionID:  &txID,
			AccountIDs:     []int64{req.FromAccountID, req.ToAccountID},
			AmountSubunits: &amt,
			Outcome:        audit.OutcomeSuccess,
		}); err != nil {
			return err
		}

		result = TransferResult{Transaction: created, Entries: []ledger.Entry{debitEntry, creditEntry}}
		return nil
	})

	if err != nil {
		var ve *validationError
		if errors.As(err, &ve) {
			return s.recordFailure(ctx, req, ve.msg, ve.code)
		}
		return TransferResult{}, fmt.Errorf("transfer: %w", err)
	}

	return result, nil
}

func (s *Service) recordFailure(ctx context.Context, req TransferRequest, reason, code string) (TransferResult, error) {
	var failedTx Transaction

	_ = pgx.BeginTxFunc(ctx, s.db, pgx.TxOptions{}, func(tx pgx.Tx) error {
		r := reason
		created, err := s.txStore.Create(ctx, tx, Transaction{
			Type:          TypeTransfer,
			Status:        StatusFailed,
			ReferenceID:   req.ReferenceID,
			FailureReason: &r,
		})
		if err != nil {
			return err
		}
		failedTx = created

		var accountIDs []int64
		if req.FromAccountID != 0 {
			accountIDs = append(accountIDs, req.FromAccountID)
		}
		if req.ToAccountID != 0 {
			accountIDs = append(accountIDs, req.ToAccountID)
		}
		amt := req.AmountSubunits
		txID := created.ID
		return s.auditStore.Insert(ctx, tx, audit.Log{
			Operation:      TypeTransfer,
			TransactionID:  &txID,
			AccountIDs:     accountIDs,
			AmountSubunits: &amt,
			Outcome:        audit.OutcomeFailure,
			FailureReason:  &r,
		})
	})

	return TransferResult{Transaction: failedTx}, &AppError{Message: reason, Code: code}
}

type validationError struct {
	msg  string
	code string
}

func (e *validationError) Error() string { return e.msg }

type AppError struct {
	Message string
	Code    string
}

func (e *AppError) Error() string { return e.Message }

func (s *Service) GetLedgerEntries(ctx context.Context, txID uuid.UUID) ([]ledger.Entry, error) {
	return s.ledgerStore.ListByTransaction(ctx, s.db, txID)
}
