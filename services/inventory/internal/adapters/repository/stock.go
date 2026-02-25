package repository

import (
	"context"
	"errors"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

type stockRepo struct {
	db postgres.PgxExecutor
}

func NewStockRepo(db postgres.PgxExecutor) ports.StockRepo {
	return &stockRepo{
		db: db,
	}
}

func (r *stockRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *stockRepo) Reserve(ctx context.Context, productID string, quantity int) error {
	sqlQuery, args, err := psql.Update((&model.Stock{}).TableName()).
		Set("reserved_quantity", squirrel.Expr("reserved_quantity + ?", quantity)).
		Set("version", squirrel.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"product_id": productID}).
		Where(squirrel.Expr("(quantity - reserved_quantity) >= ?", quantity)).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update reserve stock")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict, "insufficient stock or product not found", errors.New("optimistic lock failed: out of stock"))
	}

	return nil
}

func (r *stockRepo) Release(ctx context.Context, productID string, quantity int) error {
	sqlQuery, args, err := psql.Update((&model.Stock{}).TableName()).
		Set("reserved_quantity", squirrel.Expr("reserved_quantity - ?", quantity)).
		Set("version", squirrel.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"product_id": productID}).
		Where(squirrel.Expr("reserved_quantity >= ?", quantity)).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update reserve stock")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict, "invalid release operation: reserved stock is less than requested", errors.New("reserved stock is less than requested"))
	}

	return nil
}

func (r *stockRepo) Deduct(ctx context.Context, productID string, quantity int) error {
	sqlQuery, args, err := psql.Update((&model.Stock{}).TableName()).
		Set("quantity", squirrel.Expr("quantity - ?", quantity)).
		Set("reserved_quantity", squirrel.Expr("reserved_quantity - ?", quantity)).
		Set("version", squirrel.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"product_id": productID}).
		Where(squirrel.Expr("quantity >= ?", quantity)).
		Where(squirrel.Expr("reserved_quantity >= ?", quantity)).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update reserve stock")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict, "invalid deduct operation: state mismatch", errors.New("invalid deduct operation: state mismatch"))
	}

	return nil
}

func (r *stockRepo) Adjust(ctx context.Context, productID string, quantity int) error {
	sqlQuery, args, err := psql.Update((&model.Stock{}).TableName()).
		Set("quantity", squirrel.Expr("quantity + ?", quantity)).
		Set("version", squirrel.Expr("version + 1")).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"product_id": productID}).
		Where(squirrel.Expr("(quantity + $1) >= reserved_quantity", quantity)).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, "failed to execute update reserve stock")
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeConflict, "stock adjustment invalidates currently reserved stock", errors.New("stock adjustment invalidates currently reserved stock"))
	}

	return nil
}

func (r *stockRepo) InsertLog(ctx context.Context, stockLog *model.StockLog) error {
	queryProduct, args, err := psql.
		Insert(stockLog.TableName()).
		Columns(stockLog.ColumnsName()...).
		Values(stockLog.ToRow()...).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, queryProduct, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found, rows affected 0", pgx.ErrNoRows)
	}

	return nil
}
