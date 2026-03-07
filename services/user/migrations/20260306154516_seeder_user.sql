-- +goose Up
-- Password: 'secret_test'
-- Hash Bcrypt: $2a$10$oSfGMmaW4BwTftAiKQuswOf2MrBKt7reNMHfuF8DbitSq1HpS163q

INSERT INTO users (id, name, email, password, role, created_at, updated_at)
VALUES 
(
    gen_random_uuid(), 
    'admin', 
    'admin@example.com', 
    '$2a$10$oSfGMmaW4BwTftAiKQuswOf2MrBKt7reNMHfuF8DbitSq1HpS163q',
    'ADMIN', 
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
