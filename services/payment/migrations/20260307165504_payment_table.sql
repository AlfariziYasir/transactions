-- +goose Up
create table payments (
    id uuid primary key unique,
    order_id uuid not null,
    user_id uuid not null,
    customer_name varchar(100),
    customer_email varchar(100),
    amount decimal(15, 2) not null,
    gateway varchar(50) not null,
    method varchar(50) default '',
    reference_id varchar(100) default '',
    payment_url text default '',
    status varchar(20) not null default 'PENDING',
    paid_at timestamp,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp,
);

create index idx_payments_order_id on payments(order_id);
create index idx_payments_user_id on payments(user_id);
create index idx_payments_reference_id on payments(reference_id);

create table inbox (
    id uuid primary key unique key,
    message_id varchar(255) unique not null,
    handler_name varchar(100),
    processed_at timestamp default current_timestamp
);

create index idx_inbox_message_id on inbox(message_id);

CREATE TABLE outbox (
    id UUID PRIMARY KEY,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    payload JSONB NOT NULL,
    status VARCHAR(20) DEFAULT 'PENDING',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
