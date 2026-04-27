package audit

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) Insert(ctx context.Context, tx pgx.Tx, entry Log) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO audit_log (operation, transaction_id, account_ids, amount, outcome, failure_reason)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		entry.Operation, entry.TransactionID, entry.AccountIDs,
		entry.Amount, entry.Outcome, entry.FailureReason,
	)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

type Filters struct {
	AccountID *int64
	Outcome   *Outcome
	Operation *string
	Limit     int
	Offset    int
}

func (s *Store) List(ctx context.Context, f Filters) ([]Log, error) {
	where := []string{"1=1"}
	args := []any{}
	argIdx := 1

	if f.AccountID != nil {
		where = append(where, fmt.Sprintf("$%d = ANY(account_ids)", argIdx))
		args = append(args, *f.AccountID)
		argIdx++
	}
	if f.Outcome != nil {
		where = append(where, fmt.Sprintf("outcome = $%d", argIdx))
		args = append(args, *f.Outcome)
		argIdx++
	}
	if f.Operation != nil {
		where = append(where, fmt.Sprintf("operation = $%d", argIdx))
		args = append(args, *f.Operation)
		argIdx++
	}

	limit := 10
	if f.Limit > 0 && f.Limit <= 200 {
		limit = f.Limit
	}

	query := `SELECT id, operation, transaction_id, account_ids, amount, outcome, failure_reason, created_at
		FROM audit_log WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY created_at DESC LIMIT ` + strconv.Itoa(limit) +
		` OFFSET ` + strconv.Itoa(f.Offset)

	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list audit log: %w", err)
	}
	defer rows.Close()

	var logs []Log
	for rows.Next() {
		var l Log
		if err := rows.Scan(&l.ID, &l.Operation, &l.TransactionID, &l.AccountIDs,
			&l.Amount, &l.Outcome, &l.FailureReason, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}
