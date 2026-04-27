DELETE FROM accounts  WHERE customer_id IN (
    SELECT id FROM customers
    WHERE email LIKE '%example.com' OR email LIKE '%go-money.internal'
);
DELETE FROM customers WHERE email LIKE '%example.com' OR email LIKE '%go-money.internal';
