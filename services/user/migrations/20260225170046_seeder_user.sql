-- +goose Up
-- Password: 'password'
-- Hash Bcrypt: $2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy

INSERT INTO users (id, name, email, password, role, created_at, updated_at)
VALUES 
(
    gen_random_uuid(), 
    'admin', 
    'admin@example.com', 
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'ADMIN', 
    NOW(), 
    NOW()
) ON CONFLICT (email) DO NOTHING;

-- 2. Regular User
-- UUID: u2222222-2222-2222-2222-222222222222
INSERT INTO users (id, name, email, password, role, created_at, updated_at)
VALUES 
(
    gen_random_uuid(), 
    'user 1', 
    'user1@example.com', 
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'user', 
    NOW(), 
    NOW()
) ON CONFLICT (email) DO NOTHING;
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
