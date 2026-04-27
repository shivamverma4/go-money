package transaction

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func scanTx(row pgx.Row) (Transaction, error) {
	var t Transaction
	err := row.Scan(&t.ID, &t.Type, &t.Status, &t.ReferenceID, &t.ReversalOfID, &t.FailureReason, &t.CreatedAt)
	return t, err
}

const selectCols = `id, type, status, reference_id, reversal_of_id, failure_reason, created_at`

func (s *Store) Create(ctx context.Context, tx pgx.Tx, t Transaction) (Transaction, error) {
	created, err := scanTx(tx.QueryRow(ctx,
		`INSERT INTO transactions (type, status, reference_id, reversal_of_id, failure_reason)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING `+selectCols,
		t.Type, t.Status, t.ReferenceID, t.ReversalOfID, t.FailureReason,
	))
	if err != nil {
		return Transaction{}, fmt.Errorf("create transaction: %w", err)
	}
	return created, nil
}

func (s *Store) GetByID(ctx context.Context, id uuid.UUID) (Transaction, error) {
	t, err := scanTx(s.db.QueryRow(ctx,
		`SELECT `+selectCols+` FROM transactions WHERE id = $1`, id))
	if err != nil {
		return Transaction{}, fmt.Errorf("get transaction %s: %w", id, err)
	}
	return t, nil
}

func (s *Store) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id uuid.UUID) (Transaction, error) {
	t, err := scanTx(tx.QueryRow(ctx,
		`SELECT `+selectCols+` FROM transactions WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return Transaction{}, fmt.Errorf("lock transaction %s: %w", id, err)
	}
	return t, nil
}

func (s *Store) List(ctx context.Context, limit, offset int) ([]Transaction, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+selectCols+` FROM transactions ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var txs []Transaction
	for rows.Next() {
		t, err := scanTx(rows)
		if err != nil {
			return nil, fmt.Errorf("scan transaction: %w", err)
		}
		txs = append(txs, t)
	}
	return txs, rows.Err()
}

func (s *Store) MarkReversed(ctx context.Context, tx pgx.Tx, id uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE transactions SET status = 'reversed' WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark transaction %s reversed: %w", id, err)
	}
	return nil
}
