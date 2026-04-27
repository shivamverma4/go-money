INSERT INTO customers (name, email, kyc_status) VALUES
    ('Alice',  'alice@example.com',          'verified'),
    ('Bob',    'bob@example.com',            'verified'),
    ('Charlie','charlie@example.com',        'verified'),
    ('System', 'system@go-money.internal',   'verified');

-- Balances in paise (₹1 = 100 paise)
INSERT INTO accounts (customer_id, currency, balance, status) VALUES
    (1, 'INR', 1000000,    'active'),   -- Alice:   ₹10,000.00
    (2, 'INR',  500000,    'active'),   -- Bob:     ₹5,000.00
    (3, 'INR',  250000,    'active'),   -- Charlie: ₹2,500.00
    (4, 'INR', 9999999999, 'inactive'); -- System Treasury (internal, not user-facing)
