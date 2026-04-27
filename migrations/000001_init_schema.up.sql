CREATE TYPE account_status AS ENUM ('active', 'inactive', 'closed');

CREATE TYPE transaction_type AS ENUM ('transfer', 'reversal', 'deposit', 'withdrawal');

CREATE TYPE transaction_status AS ENUM ('completed', 'failed', 'reversed');

CREATE TYPE audit_outcome AS ENUM ('success', 'failure');

CREATE TABLE customers (
    id         BIGSERIAL    PRIMARY KEY,
    name       VARCHAR(255) NOT NULL,
    email      VARCHAR(255) NOT NULL UNIQUE,
    kyc_status VARCHAR(50)  NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE accounts (
    id          BIGSERIAL      PRIMARY KEY,
    customer_id BIGINT         NOT NULL REFERENCES customers(id),
    currency    CHAR(3)        NOT NULL DEFAULT 'INR',
    balance     NUMERIC(15,2)  NOT NULL DEFAULT 0.00,
    floor       NUMERIC(15,2)  NOT NULL DEFAULT 0.00,
    status      account_status NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);

CREATE TABLE transactions (
    id             UUID               PRIMARY KEY DEFAULT gen_random_uuid(),
    type           transaction_type   NOT NULL,
    status         transaction_status NOT NULL,
    reference_id   VARCHAR(255)       UNIQUE,
    reversal_of_id UUID               UNIQUE REFERENCES transactions(id),
    failure_reason TEXT,
    created_at     TIMESTAMPTZ        NOT NULL DEFAULT NOW()
);

CREATE TABLE ledger_entries (
    id             BIGSERIAL     PRIMARY KEY,
    transaction_id UUID          NOT NULL REFERENCES transactions(id),
    account_id     BIGINT        NOT NULL REFERENCES accounts(id),
    debit_amount   NUMERIC(15,2) NOT NULL DEFAULT 0.00 CHECK (debit_amount  >= 0),
    credit_amount  NUMERIC(15,2) NOT NULL DEFAULT 0.00 CHECK (credit_amount >= 0),
    entry_date     TIMESTAMPTZ   NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_log (
    id             BIGSERIAL        PRIMARY KEY,
    operation      transaction_type NOT NULL,
    transaction_id UUID             REFERENCES transactions(id),
    account_ids    BIGINT[],
    amount         NUMERIC(15,2),
    outcome        audit_outcome    NOT NULL,
    failure_reason TEXT,
    created_at     TIMESTAMPTZ      NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_accounts_customer  ON accounts(customer_id);
CREATE INDEX idx_ledger_transaction ON ledger_entries(transaction_id);
CREATE INDEX idx_ledger_account     ON ledger_entries(account_id);
CREATE INDEX idx_audit_transaction  ON audit_log(transaction_id);
CREATE INDEX idx_audit_created_at   ON audit_log(created_at DESC);
