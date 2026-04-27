package ledger

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Store struct{}

func NewStore() *Store { return &Store{} }

func (s *Store) InsertEntry(ctx context.Context, tx pgx.Tx, e Entry) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO ledger_entries (transaction_id, account_id, debit_amount, credit_amount)
		 VALUES ($1, $2, $3, $4)`,
		e.TransactionID, e.AccountID, e.DebitAmount, e.CreditAmount,
	)
	if err != nil {
		return fmt.Errorf("insert ledger entry: %w", err)
	}
	return nil
}

func (s *Store) ListByTransaction(ctx context.Context, db interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}, txID uuid.UUID) ([]Entry, error) {
	rows, err := db.Query(ctx,
		`SELECT id, transaction_id, account_id, debit_amount, credit_amount, entry_date
		 FROM ledger_entries WHERE transaction_id = $1 ORDER BY id`, txID)
	if err != nil {
		return nil, fmt.Errorf("list ledger entries: %w", err)
	}
	defer rows.Close()

	var entries []Entry
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.ID, &e.TransactionID, &e.AccountID, &e.DebitAmount, &e.CreditAmount, &e.EntryDate); err != nil {
			return nil, fmt.Errorf("scan ledger entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
