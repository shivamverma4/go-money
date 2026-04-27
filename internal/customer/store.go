package customer

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context) ([]Customer, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, name, email, kyc_status, created_at FROM customers ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("list customers: %w", err)
	}
	defer rows.Close()

	var customers []Customer
	for rows.Next() {
		var c Customer
		if err := rows.Scan(&c.ID, &c.Name, &c.Email, &c.KYCStatus, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan customer: %w", err)
		}
		customers = append(customers, c)
	}
	return customers, rows.Err()
}

func (s *Store) GetByID(ctx context.Context, id int64) (Customer, error) {
	var c Customer
	err := s.db.QueryRow(ctx,
		`SELECT id, name, email, kyc_status, created_at FROM customers WHERE id = $1`, id,
	).Scan(&c.ID, &c.Name, &c.Email, &c.KYCStatus, &c.CreatedAt)
	if err != nil {
		return Customer{}, fmt.Errorf("get customer %d: %w", id, err)
	}
	return c, nil
}

func (s *Store) Create(ctx context.Context, name, email string) (Customer, error) {
	var c Customer
	err := s.db.QueryRow(ctx,
		`INSERT INTO customers (name, email) VALUES ($1, $2)
		 RETURNING id, name, email, kyc_status, created_at`,
		name, email,
	).Scan(&c.ID, &c.Name, &c.Email, &c.KYCStatus, &c.CreatedAt)
	if err != nil {
		return Customer{}, fmt.Errorf("create customer: %w", err)
	}
	return c, nil
}
