package account

import "time"

type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
	StatusClosed   Status = "closed"
)

type Account struct {
	ID         int64     `json:"id"`
	CustomerID int64     `json:"customer_id"`
	Currency   string    `json:"currency"`
	Balance    float64   `json:"balance"`
	Floor      float64   `json:"floor"`
	Status     Status    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
