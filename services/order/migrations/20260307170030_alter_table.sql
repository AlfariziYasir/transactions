-- +goose Up
alter table orders
add column customer_name varchar(100),
add column customer_email varchar(100),
add column payment_url text;

create table inbox (
    id uuid primary key unique,
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
