-- +goose Up
alter table outbox
add column updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP;

alter table inbox
rename handler_name to event_name,
add constraint idx_message_id unique (message_id);

alter table payments
add column version int;
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
