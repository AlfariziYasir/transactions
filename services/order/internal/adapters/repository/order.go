package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/order/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type orderRepo struct {
	db postgres.PgxExecutor
}

func NewOrderRepo(db postgres.PgxExecutor) ports.OrderRepo {
	return &orderRepo{
		db: db,
	}
}

func (r *orderRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *orderRepo) Create(ctx context.Context, order *model.Order) error {
	query, args, _ := psql.
		Insert((&model.Order{}).TableName()).
		Columns(order.Columns()...).
		Values(order.ToRow()...).
		ToSql()

	res, err := r.getExecutor(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(
			errorx.ErrTypeInternal,
			"failed to insert order: no rows affected",
			nil,
		)
	}

	return nil
}

func (r *orderRepo) CreateBulk(ctx context.Context, columns []string, rows [][]any) error {
	count, err := r.getExecutor(ctx).CopyFrom(
		ctx,
		pgx.Identifier{(&model.OrderItem{}).TableName()},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		return err
	}

	if int(count) != len(rows) {
		return errorx.DbError(fmt.Errorf("copy count mismatch, expected %d got %d", len(rows), count), "something was wrong")
	}

	return nil
}

func (r *orderRepo) Get(ctx context.Context, filters map[string]any, order *model.Order) error {
	query := psql.Select("*").From(order.TableName())
	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	sqlQuery, args, _ := query.ToSql()
	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Order])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*order = *res

	return nil
}

func (r *orderRepo) GetDetail(ctx context.Context, orderID string) ([]*model.OrderItem, error) {
	query := psql.Select("*").From((&model.OrderItem{}).TableName()).Where(squirrel.Eq{"order_id": orderID})

	sqlQuery, args, _ := query.ToSql()
	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	items, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.OrderItem])
	if err != nil {
		return nil, errorx.DbError(err, "failed to collect rows")
	}
	return items, nil
}

func (r *orderRepo) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Order, int, error) {
	query := psql.Select("*").From((&model.Order{}).TableName())
	if len(filters) > 0 {
		for k, v := range filters {
			query = query.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
		}
	}

	sqlQuery, args, err := query.Limit(limit).Offset(offset).ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build data query", err)
	}

	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to execute list query")
	}
	defer rows.Close()

	orders, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Order])
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to collect rows")
	}

	if len(orders) == 0 {
		orders = []*model.Order{}
	}

	query2 := psql.Select("count(*)").From((&model.Order{}).TableName())
	if len(filters) > 0 {
		for k, v := range filters {
			query2 = query2.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
		}
	}

	sqlQuery2, args, err := query2.ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build count query", err)
	}

	var count int
	err = r.getExecutor(ctx).QueryRow(ctx, sqlQuery2, args...).Scan(&count)
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to count data")
	}

	return orders, count, nil
}

func (r *orderRepo) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	sqlQuery, args, err := psql.Update((&model.Order{}).TableName()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"version": currentVersion}).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(
			errorx.ErrTypeValidation,
			"failed to update order: record not found or version conflict (data modified by another process)",
			pgx.ErrNoRows,
		)
	}

	return nil
}

func (r *orderRepo) Delete(ctx context.Context, id string) error {
	sqlQuery, args, _ := psql.Update((&model.Order{}).TableName()).
		Set("deleted_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil}).
		ToSql()

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute delete")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found or already deleted", pgx.ErrNoRows)
	}

	return nil
}
