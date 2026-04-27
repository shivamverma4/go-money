package reversal_test

import (
	"context"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/shivamverma/go-money/internal/account"
	"github.com/shivamverma/go-money/internal/audit"
	"github.com/shivamverma/go-money/internal/ledger"
	"github.com/shivamverma/go-money/internal/reversal"
	"github.com/shivamverma/go-money/internal/transaction"
)

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

func seedAccounts(t *testing.T, pool *pgxpool.Pool, balanceA, balanceB int64) (int64, int64) {
	t.Helper()
	ctx := context.Background()

	var custID int64
	if err := pool.QueryRow(ctx,
		`INSERT INTO customers (name, email) VALUES ('Test', $1) RETURNING id`,
		"test-rev-"+t.Name()+"@example.com",
	).Scan(&custID); err != nil {
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

func newTxService(pool *pgxpool.Pool) *transaction.Service {
	return transaction.NewService(
		pool,
		account.NewStore(pool),
		transaction.NewStore(pool),
		ledger.NewStore(),
		audit.NewStore(pool),
	)
}

func newReversalService(pool *pgxpool.Pool) *reversal.Service {
	return reversal.NewService(
		pool,
		account.NewStore(pool),
		transaction.NewStore(pool),
		ledger.NewStore(),
		audit.NewStore(pool),
	)
}

// doTransfer is a test helper that runs a transfer and fatals on unexpected error.
func doTransfer(t *testing.T, pool *pgxpool.Pool, fromID, toID, amount int64) transaction.Transaction {
	t.Helper()
	svc := newTxService(pool)
	result, err := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  fromID,
		ToAccountID:    toID,
		AmountSubunits: amount,
	})
	if err != nil {
		t.Fatalf("setup transfer failed: %v", err)
	}
	return result.Transaction
}

// ── Tests ──────────────────────────────────────────────────────────────────

func TestReversal_HappyPath(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 50000)

	svc := newReversalService(pool)
	result, err := svc.Reverse(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Transaction.Status != transaction.StatusCompleted {
		t.Errorf("expected completed reversal, got %s", result.Transaction.Status)
	}
	if *result.Transaction.ReversalOfID != orig.ID {
		t.Errorf("reversal_of_id mismatch")
	}

	// Balances should be back to original.
	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)
	if a.Balance != 100000 {
		t.Errorf("A balance after reversal: got %d, want 100000", a.Balance)
	}
	if b.Balance != 0 {
		t.Errorf("B balance after reversal: got %d, want 0", b.Balance)
	}

	// Original transaction must now be marked reversed.
	txStore := transaction.NewStore(pool)
	updated, _ := txStore.GetByID(context.Background(), orig.ID)
	if updated.Status != transaction.StatusReversed {
		t.Errorf("original not marked reversed: got %s", updated.Status)
	}
}

func TestReversal_DoubleReversal(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 50000)

	svc := newReversalService(pool)
	if _, err := svc.Reverse(context.Background(), orig.ID); err != nil {
		t.Fatalf("first reversal failed: %v", err)
	}

	// Second reversal must fail.
	_, err := svc.Reverse(context.Background(), orig.ID)
	if err == nil {
		t.Fatal("expected error on double reversal, got nil")
	}
}

func TestReversal_OfFailedTransaction(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100, 0)
	svc := newTxService(pool)

	// This transfer will fail (insufficient funds).
	result, _ := svc.Transfer(context.Background(), transaction.TransferRequest{
		FromAccountID:  idA,
		ToAccountID:    idB,
		AmountSubunits: 9999,
	})

	revSvc := newReversalService(pool)
	_, err := revSvc.Reverse(context.Background(), result.Transaction.ID)
	if err == nil {
		t.Fatal("expected error when reversing a failed transaction")
	}
}

func TestReversal_InsufficientFundsAtDestination(t *testing.T) {
	pool := testDB(t)
	// A has 100000, B has 0. Transfer 100000 from A→B, then B spends it all.
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 100000)

	// Drain B by transferring back to A.
	doTransfer(t, pool, idB, idA, 100000)

	// Now B has 0 — reversal should fail.
	svc := newReversalService(pool)
	_, err := svc.Reverse(context.Background(), orig.ID)
	if err == nil {
		t.Fatal("expected error when destination has insufficient funds for reversal")
	}
}

func TestReversal_Concurrent(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 50000)

	svc := newReversalService(pool)
	const goroutines = 20
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		i := i
		go func() {
			defer wg.Done()
			_, errs[i] = svc.Reverse(context.Background(), orig.ID)
		}()
	}
	wg.Wait()

	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Errorf("expected exactly 1 successful reversal, got %d", successes)
	}

	// Balances must be consistent (A back to 100000, B back to 0).
	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)
	if a.Balance+b.Balance != 100000 {
		t.Errorf("total balance not conserved: A=%d B=%d", a.Balance, b.Balance)
	}
}

// ── Additional coverage ────────────────────────────────────────────────────

func TestReversal_NotFound(t *testing.T) {
	pool := testDB(t)
	svc := newReversalService(pool)

	nonExistent, _ := uuid.Parse("00000000-0000-0000-0000-000000000001")
	_, err := svc.Reverse(context.Background(), nonExistent)
	if err == nil {
		t.Fatal("expected error for non-existent transaction")
	}
}

func TestReversal_OfReversal(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 50000)

	revSvc := newReversalService(pool)
	reversal, err := revSvc.Reverse(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("first reversal failed: %v", err)
	}

	// Attempt to reverse the reversal itself — must be rejected.
	_, err = revSvc.Reverse(context.Background(), reversal.Transaction.ID)
	if err == nil {
		t.Fatal("expected error when reversing a reversal")
	}
}

func TestReversal_AuditLogCreated(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 50000)

	svc := newReversalService(pool)
	result, err := svc.Reverse(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	var count int
	if err := pool.QueryRow(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE transaction_id = $1 AND operation = 'reversal' AND outcome = 'success'`,
		result.Transaction.ID,
	).Scan(&count); err != nil {
		t.Fatalf("query audit log: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 reversal audit entry, got %d", count)
	}
}

func TestReversal_LedgerEntriesBalance(t *testing.T) {
	pool := testDB(t)
	const amount = int64(75000)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, amount)

	svc := newReversalService(pool)
	result, err := svc.Reverse(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	if len(result.Entries) != 2 {
		t.Fatalf("expected 2 ledger entries for reversal, got %d", len(result.Entries))
	}
	var debitSum, creditSum int64
	for _, e := range result.Entries {
		debitSum += e.DebitAmount
		creditSum += e.CreditAmount
	}
	if debitSum != creditSum {
		t.Errorf("reversal ledger imbalance: debit=%d credit=%d", debitSum, creditSum)
	}
	if debitSum != amount {
		t.Errorf("reversal amount mismatch: got %d, want %d", debitSum, amount)
	}
}

func TestReversal_OriginalMarkedReversed(t *testing.T) {
	pool := testDB(t)
	idA, idB := seedAccounts(t, pool, 100000, 0)
	orig := doTransfer(t, pool, idA, idB, 10000)

	svc := newReversalService(pool)
	if _, err := svc.Reverse(context.Background(), orig.ID); err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	txStore := transaction.NewStore(pool)
	updated, err := txStore.GetByID(context.Background(), orig.ID)
	if err != nil {
		t.Fatalf("get original tx: %v", err)
	}
	if updated.Status != transaction.StatusReversed {
		t.Errorf("expected original status 'reversed', got %s", updated.Status)
	}
}

func TestReversal_TotalBalanceConserved(t *testing.T) {
	pool := testDB(t)
	const initial = int64(100000)
	idA, idB := seedAccounts(t, pool, initial, 0)
	orig := doTransfer(t, pool, idA, idB, 60000)

	svc := newReversalService(pool)
	if _, err := svc.Reverse(context.Background(), orig.ID); err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	accStore := account.NewStore(pool)
	a, _ := accStore.GetByID(context.Background(), idA)
	b, _ := accStore.GetByID(context.Background(), idB)
	if a.Balance+b.Balance != initial {
		t.Errorf("balance not conserved after reversal: A=%d B=%d sum=%d want=%d",
			a.Balance, b.Balance, a.Balance+b.Balance, initial)
	}
	if a.Balance != initial {
		t.Errorf("A not fully restored: got %d, want %d", a.Balance, initial)
	}
}
