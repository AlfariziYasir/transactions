package repository

import (
	"context"
	"fmt"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/payment/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type paymentRepo struct {
	db postgres.PgxExecutor
}

func NewPaymentRepo(db postgres.PgxExecutor) ports.PaymentRepository {
	return &paymentRepo{db: db}
}

func (r *paymentRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *paymentRepo) Create(ctx context.Context, payment *model.Payment) error {
	query, args, _ := psql.Insert(payment.TableName()).
		Columns(payment.ColumnsName()...).
		Values(payment.ToRow()...).
		ToSql()

	res, err := r.getExecutor(ctx).Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(
			errorx.ErrTypeInternal,
			"failed to insert payment: no rows affected",
			nil,
		)
	}

	return nil
}

func (r *paymentRepo) Get(ctx context.Context, filters map[string]any, payment *model.Payment) error {
	query := psql.Select("*").From(payment.TableName())
	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	sqlQuery, args, _ := query.ToSql()
	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.Payment])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*payment = *res

	return nil
}

func (r *paymentRepo) GetStatus(ctx context.Context, duration int) ([]*model.Payment, error) {
	sqlQuery, args, _ := psql.Select("*").From((&model.Payment{}).TableName()).
		Where(squirrel.Eq{"status": string(model.PaymentStatusPending)}).
		Where(squirrel.Expr("created_at <= now() - interval '? minutes'", duration)).
		ToSql()

	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	payments, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Payment])
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}

	return payments, nil
}

func (r *paymentRepo) Update(ctx context.Context, id string, currentVersion int, data map[string]any) error {
	sqlQuery, args, err := psql.Update((&model.Payment{}).TableName()).
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
			"failed to update payment: record not found or version conflict (data modified by another process)",
			pgx.ErrNoRows,
		)
	}

	return nil
}

func (r *paymentRepo) List(ctx context.Context, limit, offset uint64, filters map[string]interface{}) ([]*model.Payment, int, error) {
	query := psql.Select("*").From((&model.Payment{}).TableName())
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

	orders, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Payment])
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to collect rows")
	}

	if len(orders) == 0 {
		orders = []*model.Payment{}
	}

	query2 := psql.Select("count(*)").From((&model.Payment{}).TableName())
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
