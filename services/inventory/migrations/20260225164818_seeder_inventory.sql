-- +goose Up
INSERT INTO products (id, sku, name, description, price, is_active, created_at, updated_at)
VALUES 
(
    '9a7b69d8-8756-4322-a567-9f1234567890', 
    'LAPTOP-001', 
    'MacBook Pro M3 Max', 
    'Apple M3 Max chip with 14‑core CPU, 30‑core GPU, 36GB Unified Memory, 1TB SSD Storage', 
    35000000, 
    true, 
    NOW(), 
    NOW()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO stocks (product_id, quantity, reserved_quantity, version, updated_at)
VALUES 
(
    '9a7b69d8-8756-4322-a567-9f1234567890', 
    10, 
    0, 
    1, 
    NOW()
) ON CONFLICT (product_id) DO NOTHING;

INSERT INTO products (id, sku, name, description, price, is_active, created_at, updated_at)
VALUES 
(
    'b249e053-6a3f-459e-9538-234567890123', 
    'KEYBOARD-001', 
    'Keychron Q1 Pro', 
    'Wireless Custom Mechanical Keyboard, QMK/VIA Support, Full Aluminum Body', 
    2800000, 
    true, 
    NOW(), 
    NOW()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO stocks (product_id, quantity, reserved_quantity, version, updated_at)
VALUES 
(
    'b249e053-6a3f-459e-9538-234567890123', 
    50, 
    0, 
    1, 
    NOW()
) ON CONFLICT (product_id) DO NOTHING;

INSERT INTO products (id, sku, name, description, price, is_active, created_at, updated_at)
VALUES 
(
    'c81d4e2e-bcf2-11e0-962b-0800200c9a66', 
    'MOUSE-001', 
    'Logitech G Pro X Superlight 2', 
    'Ultra-lightweight wireless gaming mouse, HERO 2 Sensor', 
    2100000, 
    true, 
    NOW(), 
    NOW()
) ON CONFLICT (id) DO NOTHING;

INSERT INTO stocks (product_id, quantity, reserved_quantity, version, updated_at)
VALUES 
(
    'c81d4e2e-bcf2-11e0-962b-0800200c9a66', 
    5, 
    0, 
    1, 
    NOW()
) ON CONFLICT (product_id) DO NOTHING;
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
