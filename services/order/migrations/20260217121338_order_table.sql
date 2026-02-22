-- +goose Up
create table orders (
    id uuid primary key not null unique,
    user_id uuid not null,
    total_amount decimal(15,2) not null,
    currency varchar(3) default 'IDR',
    status varchar(20) not null,
    shipping_address text not null,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp,
    version int default 1
);
create index orders_idx on orders(id, user_id);

create table order_items (
    id uuid primary key not null unique,
    order_id uuid references orders(id),
    product_id uuid not null,
    product_name varchar(255) not null,
    quantity int not null,
    price decimal(15,2) not null,
    subtotal decimal(15,2) not null
);
create index order_item_idx on order_items(id, order_id);

create table outbox (
    id uuid primary key,
    aggregate_type varchar(50) not null,
    aggregate_id uuid not null,
    event_type varchar(50) not null,
    payload jsonb not null,
    status varchar(20) default 'PENDING',
    created_at timestamp default current_timestamp
);

create table product_replicas (
    id uuid primary key not null unique,
    name varchar(255) not null,
    price decimal(15,2) not null,
    is_active boolean default true,
    last_updated timestamp not null,
    version int
);
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
