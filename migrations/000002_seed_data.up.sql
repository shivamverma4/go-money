INSERT INTO customers (name, email, kyc_status) VALUES
    ('Alice',  'alice@example.com',          'verified'),
    ('Bob',    'bob@example.com',            'verified'),
    ('Charlie','charlie@example.com',        'verified'),
    ('System', 'system@go-money.internal',   'verified');

-- Balances stored in rupees
INSERT INTO accounts (customer_id, currency, balance, status) VALUES
    (1, 'INR',    10000.00, 'active'),   -- Alice:   ₹10,000.00
    (2, 'INR',     5000.00, 'active'),   -- Bob:     ₹5,000.00
    (3, 'INR',     2500.00, 'active'),   -- Charlie: ₹2,500.00
    (4, 'INR', 99999999.99, 'inactive'); -- System Treasury (internal, not user-facing)
