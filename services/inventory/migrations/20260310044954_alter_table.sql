-- +goose Up
alter table outbox
add column updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
