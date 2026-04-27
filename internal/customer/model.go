package customer

import "time"

type Customer struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	KYCStatus string    `json:"kyc_status"`
	CreatedAt time.Time `json:"created_at"`
}
