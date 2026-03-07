package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/inventory/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type productRepo struct {
	db postgres.PgxExecutor
}

func NewProductRepo(db postgres.PgxExecutor) ports.ProductRepo {
	return &productRepo{
		db: db,
	}
}

func (r *productRepo) getExecutor(ctx context.Context) postgres.PgxExecutor {
	tx, ok := ctx.Value(postgres.TrxKey{}).(pgx.Tx)
	if ok {
		return tx
	}

	return r.db
}

func (r *productRepo) Create(ctx context.Context, product *model.Product, initialStock int) error {
	queryProduct, args, err := psql.
		Insert(product.TableName()).
		Columns(product.ColumnsName()...).
		Values(product.ToRow()...).
		ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.getExecutor(ctx).Exec(ctx, queryProduct, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found, rows affected 0", pgx.ErrNoRows)
	}

	stock := &model.Stock{
		ProductID:        product.ID,
		Quantity:         initialStock,
		ReservedQuantity: 0,
		Version:          1,
		UpdatedAt:        time.Now(),
	}
	queryStock, args, _ := psql.
		Insert(stock.TableName()).
		Columns(stock.ColumnsName()...).
		Values(stock.ToRow()...).
		ToSql()

	res, err = r.getExecutor(ctx).Exec(ctx, queryStock, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found, rows affected 0", pgx.ErrNoRows)
	}

	return nil
}

func (r *productRepo) Get(ctx context.Context, filters map[string]any, product *model.ProductWithStock) error {
	query := psql.Select(
		"products.id",
		"products.sku",
		"products.name",
		"products.description",
		"products.price",
		"products.is_active",
		"products.created_at product_created_at",
		"products.updated_at product_updated_at",
		"stocks.quantity",
		"stocks.reserved_quantity",
		"stocks.version",
		"stocks.updated_at stock_updated_at").
		From((&model.Product{}).TableName()).
		LeftJoin(fmt.Sprintf("%s on products.id = stocks.product_id", (&model.Stock{}).TableName()))

	for k, v := range filters {
		query = query.Where(squirrel.Eq{fmt.Sprintf("products.%s", k): v})
	}

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.ProductWithStock])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*product = *res

	return nil
}

func (r *productRepo) GetByIDs(ctx context.Context, ids []string) ([]*model.ProductWithStock, error) {
	query := psql.Select(
		"products.id",
		"products.sku",
		"products.name",
		"products.description",
		"products.price",
		"products.is_active",
		"products.created_at product_created_at",
		"products.updated_at product_updated_at",
		"stocks.quantity",
		"stocks.reserved_quantity",
		"stocks.version",
		"stocks.updated_at stock_updated_at").
		From((&model.Product{}).TableName()).
		LeftJoin(fmt.Sprintf("%s on products.id = stocks.product_id", (&model.Stock{}).TableName())).
		Where(squirrel.Eq{"products.id": ids})

	sqlQuery, args, err := query.ToSql()
	if err != nil {
		return nil, errorx.NewError(errorx.ErrTypeInternal, "failed to build query", err)
	}

	rows, err := r.getExecutor(ctx).Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	products, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.ProductWithStock])
	if err != nil {
		return nil, errorx.DbError(err, "failed to collect rows")
	}
	return products, nil
}

func (r *productRepo) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.Product, int, error) {
	query := psql.Select("*").From((&model.Product{}).TableName())
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

	products, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.Product])
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to collect rows")
	}

	if len(products) == 0 {
		products = []*model.Product{}
	}

	query2 := psql.Select("count(*)").From((&model.Product{}).TableName())
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

	return products, count, nil
}

func (r *productRepo) Update(ctx context.Context, id string, data map[string]any) error {
	sqlQuery, args, err := psql.Update((&model.Product{}).TableName()).
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

func (r *productRepo) Delete(ctx context.Context, id string) error {
	sqlQuery, args, _ := psql.Update((&model.Product{}).TableName()).
		Set("is_active", false).
		Set("updated_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"is_active": true}).
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
