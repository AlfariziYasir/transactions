package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/AlfariziYasir/transactions/common/pkg/errorx"
	"github.com/AlfariziYasir/transactions/common/pkg/postgres"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/model"
	"github.com/AlfariziYasir/transactions/services/user/internal/core/ports"
	"github.com/Masterminds/squirrel"
	"github.com/jackc/pgx/v5"
)

var psql = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)

type repository struct {
	db postgres.PgxExecutor
}

func NewRepository(db postgres.PgxExecutor) ports.Repository {
	return &repository{
		db: db,
	}
}

func (r *repository) Create(ctx context.Context, user *model.User) error {
	query, args, _ := psql.
		Insert(user.Tablename()).
		Columns(user.ColumnsName()...).
		Values(user.ToRow()...).
		ToSql()

	res, err := r.db.Exec(ctx, query, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}

func (r *repository) Get(ctx context.Context, filters map[string]any, status bool, user *model.User) error {
	query := psql.Select((&model.User{}).ColumnsName()...).From((&model.User{}).Tablename())
	for k, v := range filters {
		query = query.Where(squirrel.Eq{k: v})
	}

	if status {
		query = query.Where(squirrel.Expr("deleted_at IS NULL"))
	}

	sqlQuery, args, _ := query.ToSql()
	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	res, err := pgx.CollectOneRow(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	*user = *res

	return nil
}

func (r *repository) List(ctx context.Context, limit, offset uint64, filters map[string]any) ([]*model.User, int, error) {
	query := psql.Select((&model.User{}).ColumnsName()...).From((&model.User{}).Tablename())
	if len(filters) > 0 {
		for k, v := range filters {
			query = query.Where(squirrel.ILike{k: fmt.Sprintf("%%%s%%", v)})
		}
	}

	sqlQuery, args, err := query.Limit(limit).Offset(offset).ToSql()
	if err != nil {
		return nil, 0, errorx.NewError(errorx.ErrTypeInternal, "failed to build data query", err)
	}

	rows, err := r.db.Query(ctx, sqlQuery, args...)
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}
	defer rows.Close()

	results, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByName[model.User])
	if err != nil {
		return nil, 0, errorx.DbError(err, err.Error())
	}

	if len(results) == 0 {
		results = []*model.User{}
	}

	query2 := psql.Select("count(*)").From((&model.User{}).Tablename())
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
	err = r.db.QueryRow(ctx, sqlQuery2, args...).Scan(&count)
	if err != nil {
		return nil, 0, errorx.DbError(err, "failed to count users")
	}

	return results, count, nil
}

func (r *repository) Update(ctx context.Context, id string, data map[string]any) error {
	sqlQuery, args, err := psql.Update((&model.User{}).Tablename()).
		SetMap(data).
		Where(squirrel.Eq{"id": id}).ToSql()
	if err != nil {
		return errorx.NewError(errorx.ErrTypeInternal, "failed to build update query", err)
	}

	res, err := r.db.Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}

func (r *repository) Delete(ctx context.Context, id string) error {
	sqlQuery, args, _ := psql.Update((&model.User{}).Tablename()).
		Set("deleted_at", time.Now()).
		Where(squirrel.Eq{"id": id}).
		Where(squirrel.Eq{"deleted_at": nil}).
		ToSql()

	res, err := r.db.Exec(ctx, sqlQuery, args...)
	if err != nil {
		return errorx.DbError(err, err.Error())
	}

	if res.RowsAffected() == 0 {
		return errorx.NewError(errorx.ErrTypeNotFound, "record not found", pgx.ErrNoRows)
	}

	return nil
}
