-- +goose Up
create table products (
    id UUID primary key unique,
    sku varchar(50) unique not null, -- Stock Keeping Unit untuk identifikasi unik barang
    name VARCHAR(255) not null,
    description text not null,
    price DECIMAL(15, 2) not null default 0,
    is_active boolean default true,
    created_at timestamp default current_timestamp,
    updated_at timestamp default current_timestamp
);
create index idx_products on products(sku, id);

create table stocks (
    product_id UUID primary key references products(id) on delete cascade,
    quantity integer not null default 0,         
    reserved_quantity integer not null default 0,
    version integer not null default 0,          
    updated_at timestamp default current_timestamp
);
alter table stocks add constraint chk_stock check (quantity >= 0);
alter table stocks add constraint chk_reserved check (reserved_quantity >= 0);

create type stock_log_type as enum ('INCOMING', 'OUTGOING', 'RESERVE', 'RELEASE', 'ADJUSTMENT');
create table stock_logs (
    id UUID primary key unique,
    product_id UUID references products(id),
    type stock_log_type not null,
    quantity integer not null,
    reference_id varchar(100),
    reason text,
    created_at timestamp default current_timestamp
);
create index idx_stock_logs ON stock_logs(product_id, id);

create table outbox (
    id uuid primary key,
    aggregate_type varchar(50) not null,
    aggregate_id uuid not null,
    event_type varchar(50) not null,
    payload jsonb not null,
    status varchar(20) default 'PENDING',
    created_at timestamp default current_timestamp
);
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
