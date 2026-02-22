package repository

import (
	"context"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type outboxRepo struct {
	db postgres.PgxExecutor
}

func NewOutboxRepo(db postgres.PgxExecutor) ports.OutboxRepo {
	return &outboxRepo{
		db: db,
	}
}

func (r *outboxRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *outboxRepo) Create(ctx context.Context, data map[string]any) error {
	query, args, _ := psql.
		Insert((&model.Outbox{}).TableName()).
		SetMap(data).
		Suffix("returning id").
		ToSql()

	var id string
	err := r.getExecutor(ctx).QueryRow(ctx, query, args...).Scan(&id)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	return nil
}

func (r *outboxRepo) Get(ctx context.Context, limit uint64) ([]*model.Outbox, error) {
	query := psql.Select("*").From((&model.Outbox{}).TableName()).
		Where(squirrel.Eq{"status": "PENDING"}).
		Limit(limit)

	sqlQuery, args, _ := query.ToSql()
	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	outboxs, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Outbox])
	if err != nil {
		return nil, errorx.DbError(err, "failed to collect rows")
	}
	return outboxs, nil
}

func (r *outboxRepo) Update(ctx context.Context, id string, data map[string]any) error {
	sqlQuery, args, err := psql.Update((&model.Outbox{}).TableName()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}
