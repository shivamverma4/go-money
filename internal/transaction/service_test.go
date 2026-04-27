package transaction_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/audit"
	"github.com/shivamverma/go-money/internal/ledger"
	"github.com/shivamverma/go-money/internal/transaction"
)

// testDB returns a pool connected to the test database.
// Set TEST_DATABASE_URL or it falls back to the default local DB.
func testDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://postgres:password@localhost:5432/go_money_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect to test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// seedAccounts inserts two accounts with the given balances and returns their IDs.
func seedAccounts(t *testing.T, pool *pgxpool.Pool, balanceA, balanceB int64) (int64, int64) {
	t.Helper()
	ctx := context.Background()

	var custID int64
	err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email) VALUES ('Test', $1) RETURNING id`,
		"test"+t.Name()+"@example.com",
	).Scan(&custID)
	if err != nil {
		t.Fatalf("seed customer: %v", err)
	}

	var idA, idB int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (customer_id, currency, balance) VALUES ($1, 'INR', $2) RETURNING id`,
		custID, balanceA,
	).Scan(&idA); err != nil {
		t.Fatalf("seed account A: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (customer_id, currency, balance) VALUES ($1, 'INR', $2) RETURNING id`,
		custID, balanceB,
	).Scan(&idB); err != nil {
		t.Fatalf("seed account B: %v", err)
	}

	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM audit_log WHERE account_ids && ARRAY[$1,$2]::bigint[]`, idA, idB)
		pool.Exec(context.Background(), `DELETE FROM ledger_entries WHERE account_id IN ($1,$2)`, idA, idB)
		pool.Exec(context.Background(), `DELETE FROM transactions WHERE id IN (
			SELECT transaction_id FROM ledger_entries WHERE account_id IN ($1,$2))`, idA, idB)
		pool.Exec(context.Background(), `DELETE FROM accounts WHERE id IN ($1,$2)`, idA, idB)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, custID)
	})

	return idA, idB
}

func newService(pool *pgxpool.Pool) *transaction.Service {
	return transaction.NewService(
		pool,
		account.NewStore(pool),
		transaction.NewStore(pool),
		ledger.NewStore(),
		audit.NewStore(pool),
	)
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestTransfer_HappyPath(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 50000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Transaction.Status != transaction.StatusCompleted {
		t.Errorf("expected completed, got %s", result.Transaction.Status)
	}

	// Verify balances.
	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)
	if a.Balance != 50000 {
		t.Errorf("account A balance: got %d, want 50000", a.Balance)
	}
	if b.Balance != 50000 {
		t.Errorf("account B balance: got %d, want 50000", b.Balance)
	}

	// Verify ledger invariant: 2 entries, debits == credits within the tx.
	if len(result.Entries) != 2 {
		t.Errorf("expected 2 ledger entries, got %d", len(result.Entries))
	}
	var totalDebit, totalCredit int64
	for _, e := range result.Entries {
		totalDebit += e.DebitAmount
		totalCredit += e.CreditAmount
	}
	if totalDebit != totalCredit {
		t.Errorf("ledger imbalance: debit=%d credit=%d", totalDebit, totalCredit)
	}
}

func TestTransfer_InsufficientFunds(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 1000, 0)
	svc := newService(pool)

	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 9999,
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	ae, ok := err.(*transaction.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "INSUFFICIENT_FUNDS" {
		t.Errorf("expected INSUFFICIENT_FUNDS, got %s", ae.Code)
	}
	if result.Transaction.Status != transaction.StatusFailed {
		t.Errorf("expected failed transaction record, got %s", result.Transaction.Status)
	}

	// Balances must be unchanged.
	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	if a.Balance != 1000 {
		t.Errorf("balance changed on failure: got %d, want 1000", a.Balance)
	}
}

func TestTransfer_SameAccount(t *testing.T) {
	pool := testDB(t)
	idA, _ := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idA,
		AmountSubunits: 100,
	})
	if err == nil {
		t.Fatal("expected error for same-account transfer")
	}
}

func TestTransfer_ZeroAmount(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 0,
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
}

func TestTransfer_AccountNotFound(t *testing.T) {
	pool := testDB(t)
	_, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  999999999,
		ToAccountID:    idB,
		AmountSubunits: 100,
	})
	if err == nil {
		t.Fatal("expected error for missing account")
	}
}

// TestTransfer_Concurrent spawns N goroutines all transferring from A to B
// simultaneously. Asserts no balance goes negative and total is conserved.
func TestTransfer_Concurrent(t *testing.T) {
	pool := testDB(t)
	const (
		initial    = int64(1_000_000) // ₹10,000
		perTx      = int64(1_000)     // ₹10 each
		goroutines = 50
	)
	idA, idB := seedAccounts(t, pool, initial, 0)
	svc := newService(pool)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			svc.Transfer(context.Background(), transaction.TransferRequest{
				FromAccountID:  idA,
				ToAccountID:    idB,
				AmountSubunits: perTx,
			})
		}()
	}
	wg.Wait()

	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)

	if a.Balance < 0 {
		t.Errorf("account A went negative: %d", a.Balance)
	}
	if a.Balance+b.Balance != initial {
		t.Errorf("total balance not conserved: A=%d B=%d sum=%d want=%d",
			a.Balance, b.Balance, a.Balance+b.Balance, initial)
	}
}

// ── Additional validation & invariant tests ────────────────────────────────

func seedInactiveAccount(t *testing.T, pool *pgxpool.Pool) (int64, int64) {
	t.Helper()
	ctx := context.Background()
	var custID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email) VALUES ('InactiveTest', $1) RETURNING id`,
		"inactive"+t.Name()+"@example.com",
	).Scan(&custID); err != nil {
		t.Fatalf("seed customer: %v", err)
	}
	var activeID, inactiveID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (customer_id, currency, balance) VALUES ($1, 'INR', 100000) RETURNING id`,
		custID,
	).Scan(&activeID); err != nil {
		t.Fatalf("seed active account: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`INSERT INTO accounts (customer_id, currency, balance, status) VALUES ($1, 'INR', 100000, 'inactive') RETURNING id`,
		custID,
	).Scan(&inactiveID); err != nil {
		t.Fatalf("seed inactive account: %v", err)
	}
	t.Cleanup(func() {
		pool.Exec(context.Background(), `DELETE FROM audit_log WHERE account_ids && ARRAY[$1,$2]::bigint[]`, activeID, inactiveID)
		pool.Exec(context.Background(), `DELETE FROM ledger_entries WHERE account_id IN ($1,$2)`, activeID, inactiveID)
		pool.Exec(context.Background(), `DELETE FROM transactions WHERE id IN (SELECT transaction_id FROM ledger_entries WHERE account_id IN ($1,$2))`, activeID, inactiveID)
		pool.Exec(context.Background(), `DELETE FROM accounts WHERE id IN ($1,$2)`, activeID, inactiveID)
		pool.Exec(context.Background(), `DELETE FROM customers WHERE id = $1`, custID)
	})
	return activeID, inactiveID
}

func TestTransfer_InactiveSourceAccount(t *testing.T) {
	pool := testDB(t)
	activeID, inactiveID := seedInactiveAccount(t, pool)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  inactiveID,
		ToAccountID:    activeID,
		AmountSubunits: 100,
	})
	if err == nil {
		t.Fatal("expected error for inactive source account")
	}
	ae, ok := err.(*transaction.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "ACCOUNT_NOT_ACTIVE" {
		t.Errorf("expected ACCOUNT_NOT_ACTIVE, got %s", ae.Code)
	}
}

func TestTransfer_InactiveDestinationAccount(t *testing.T) {
	pool := testDB(t)
	activeID, inactiveID := seedInactiveAccount(t, pool)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  activeID,
		ToAccountID:    inactiveID,
		AmountSubunits: 100,
	})
	if err == nil {
		t.Fatal("expected error for inactive destination account")
	}
	ae, ok := err.(*transaction.AppError)
	if !ok {
		t.Fatalf("expected AppError, got %T", err)
	}
	if ae.Code != "ACCOUNT_NOT_ACTIVE" {
		t.Errorf("expected ACCOUNT_NOT_ACTIVE, got %s", ae.Code)
	}
}

func TestTransfer_NegativeAmount(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	_, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: -500,
	})
	if err == nil {
		t.Fatal("expected error for negative amount")
	}
}

func TestTransfer_AuditLogOnSuccess(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	svc := newService(pool)

	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 10000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int
	err = pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE transaction_id = $1 AND outcome = 'success'`,
		result.Transaction.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 audit log entry on success, got %d", count)
	}
}

func TestTransfer_AuditLogOnFailure(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100, 0)
	svc := newService(pool)

	result, _ := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 9999,
	})

	var count int
	err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE transaction_id = $1 AND outcome = 'failure'`,
		result.Transaction.ID,
	).Scan(&count)
	if err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 failure audit log entry, got %d", count)
	}
}

func TestTransfer_LedgerBalanceConservation(t *testing.T) {
	pool := testDB(t)
	const initial = int64(500000)
	idA, idB := seedAccounts(t, pool, initial, 0)
	svc := newService(pool)

	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 200000,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total balance must equal original initial.
	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)
	if a.Balance+b.Balance != initial {
		t.Errorf("balance not conserved: got %d+%d=%d, want %d",
			a.Balance, b.Balance, a.Balance+b.Balance, initial)
	}

	// Ledger must balance: sum(debits) == sum(credits) within the transaction.
	var debitSum, creditSum int64
	for _, e := range result.Entries {
		debitSum += e.DebitAmount
		creditSum += e.CreditAmount
	}
	if debitSum != creditSum {
		t.Errorf("ledger imbalance: debit=%d credit=%d", debitSum, creditSum)
	}
}

func TestTransfer_BalanceFloor(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 50000, 0)
	// Set floor to 10000 so effective transferable is only 40000.
	_, err := pool.Exec(context.Background(),
		`UPDATE accounts SET floor = 10000 WHERE id = $1`, idA)
	if err != nil {
		t.Fatalf("set floor: %v", err)
	}
	svc := newService(pool)

	// Transfer of 45000 would bring balance to 5000, below floor 10000 — must fail.
	_, err = svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 45000,
	})
	if err == nil {
		t.Fatal("expected insufficient funds when transfer would breach floor")
	}

	// Transfer of 40000 is exactly at floor — must succeed.
	_, err = svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 40000,
	})
	if err != nil {
		t.Errorf("expected success at floor boundary, got: %v", err)
	}
}
