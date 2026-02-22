-- +goose Up
create table users (
    id uuid primary key unique not null,
    name varchar(50) not null,
    email varchar(100) not null,
    role varchar(10) not null,
    password text not null,
    created_at timestamp default current_timestamp,
    updated_at timestamp,
    deleted_at timestamp
)
-- +goose StatementBegin
SELECT 'up SQL query';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
SELECT 'down SQL query';
-- +goose StatementEnd
