-- +goose Up
create table inbox (
    id uuid primary key unique key,
    message_id varchar(255) unique not null,
    handler_name varchar(100),
    processed_at timestamp default current_timestamp
);

create index idx_inbox_message_id on inbox(message_id);
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
