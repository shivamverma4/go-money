# go-money

Simple banking ledger — transfers, reversals, audit log. Go backend + React frontend, single Docker image.

## Running

```bash
docker compose up --build
```

Open http://localhost:8080. Migrations and seed data run automatically on first start.

**Local dev (without Docker)**

You need Go 1.25+, Node 22, Postgres 16, and golang-migrate.

```bash
export DATABASE_URL="postgres://go_money:go_money_secret@localhost:5432/go_money?sslmode=disable"
migrate -path ./migrations -database "$DATABASE_URL" up
go run ./cmd/server

cd frontend && npm install && npm run dev  # http://localhost:5173
```

## Migrations

```bash
migrate -path ./migrations -database "$DATABASE_URL" up
migrate -path ./migrations -database "$DATABASE_URL" down 1
```

Two files: `000001` creates the schema, `000002` seeds demo accounts (Alice ₹10k, Bob ₹5k, Charlie ₹2.5k).

## Tests

Backend tests hit a real database — no mocks.

```bash
docker run -d --name pg-test \
  -e POSTGRES_DB=go_money_test -e POSTGRES_USER=postgres -e POSTGRES_PASSWORD=password \
  -p 5433:5432 postgres:16-alpine

migrate -path ./migrations \
  -database "postgres://postgres:password@localhost:5433/go_money_test?sslmode=disable" up 1

export TEST_DATABASE_URL="postgres://postgres:password@localhost:5433/go_money_test?sslmode=disable"
go test -race ./...
```

Frontend tests don't need a database:
```bash
cd frontend && npm test -- --run
```

## Concurrency testing
Uses [hey](https://github.com/rakyll/hey). Install it:

```bash
go install github.com/rakyll/hey@latest
export PATH=$PATH:~/go/bin
```

Run against a live app (`docker compose up --build` first):

```bash
# low concurrency
hey -n 500 -c 20 -m POST \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":1,"to_account_id":2,"amount":1.00}' \
  http://localhost:8080/api/transactions

# high concurrency
hey -n 1000 -c 200 -m POST \
  -H "Content-Type: application/json" \
  -d '{"from_account_id":1,"to_account_id":2,"amount":1.00}' \
  http://localhost:8080/api/transactions
```

## API

```
GET  /api/accounts
POST /api/accounts

GET  /api/transactions
POST /api/transactions          { from_account_id, to_account_id, amount }
POST /api/transactions/:id/reverse

GET  /api/audit-log             ?account_id=&outcome=&limit=&offset=
GET  /api/customers
```


## Design notes

**Decimal balances.** Stored as `NUMERIC(15,2)` in Postgres — decimal rupees, not paise. ₹10,000 = `10000.00`. Scanned into `float64` in Go. Display formatting (₹1,000.00 with Indian comma notation) happens server-side in the HTTP response.

**Locking.** Transfers lock both account rows with `SELECT FOR UPDATE` inside a single transaction. Accounts are always locked in ascending ID order to prevent deadlocks when two transfers touch the same pair in opposite directions.

**Reversal idempotency.** `transactions.reversal_of_id` has a `UNIQUE` constraint. Even if the application check is somehow bypassed, the DB will reject a second reversal. Maps to a 409.

**Audit log.** Failures are recorded too, in a separate short transaction. Every attempted operation has a trace — the log has no gaps.

**Double-entry ledger.** Every transfer writes two rows: debit from source, credit to destination. `SUM(debit) == SUM(credit)` always. Reversals write a mirrored pair. The table is append-only.

**Single binary.** The Docker image bundles the compiled React app into the Go binary's static file server (`SERVE_STATIC=true`). No nginx, no separate container needed.
