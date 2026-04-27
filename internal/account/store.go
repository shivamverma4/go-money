package account

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

const selectCols = `id, customer_id, currency, balance, floor, status, created_at, updated_at`

func scan(row pgx.Row) (Account, error) {
	var a Account
	err := row.Scan(&a.ID, &a.CustomerID, &a.Currency, &a.Balance, &a.Floor, &a.Status, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (s *Store) List(ctx context.Context) ([]Account, error) {
	rows, err := s.db.Query(ctx, `SELECT `+selectCols+` FROM accounts ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) GetByID(ctx context.Context, id int64) (Account, error) {
	a, err := scan(s.db.QueryRow(ctx,
		`SELECT `+selectCols+` FROM accounts WHERE id = $1`, id))
	if err != nil {
		return Account{}, fmt.Errorf("get account %d: %w", id, err)
	}
	return a, nil
}

// GetByIDForUpdate locks the row within an existing transaction.
func (s *Store) GetByIDForUpdate(ctx context.Context, tx pgx.Tx, id int64) (Account, error) {
	a, err := scan(tx.QueryRow(ctx,
		`SELECT `+selectCols+` FROM accounts WHERE id = $1 FOR UPDATE`, id))
	if err != nil {
		return Account{}, fmt.Errorf("lock account %d: %w", id, err)
	}
	return a, nil
}

func (s *Store) ListByCustomer(ctx context.Context, customerID int64) ([]Account, error) {
	rows, err := s.db.Query(ctx,
		`SELECT `+selectCols+` FROM accounts WHERE customer_id = $1 ORDER BY id`, customerID)
	if err != nil {
		return nil, fmt.Errorf("list accounts for customer %d: %w", customerID, err)
	}
	defer rows.Close()

	var accounts []Account
	for rows.Next() {
		a, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("scan account: %w", err)
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

func (s *Store) Create(ctx context.Context, customerID int64, currency string, initialBalance float64) (Account, error) {
	a, err := scan(s.db.QueryRow(ctx,
		`INSERT INTO accounts (customer_id, currency, balance)
		 VALUES ($1, $2, $3)
		 RETURNING `+selectCols,
		customerID, currency, initialBalance,
	))
	if err != nil {
		return Account{}, fmt.Errorf("create account: %w", err)
	}
	return a, nil
}

// UpdateBalance applies a signed delta to balance within an existing transaction.
func (s *Store) UpdateBalance(ctx context.Context, tx pgx.Tx, id int64, delta float64) error {
	_, err := tx.Exec(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = NOW() WHERE id = $2`,
		delta, id,
	)
	if err != nil {
		return fmt.Errorf("update balance for account %d: %w", id, err)
	}
	return nil
}
